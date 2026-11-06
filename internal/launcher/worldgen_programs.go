package launcher

import (
	"archive/zip"
	"errors"
	"fmt"
	"maps"
	"os"
	"runtime"
	"sync"

	"github.com/Viking602/YoloMCL/cubiomes/modjar"
)

// Registration runs after the complete mod/resource catalogue is available.
// Optional integrations need the whole installed-mod set, and registry queries
// must see the effective datapacks rather than whichever JAR happened to be read
// first. Discovery is parallel; registration/configuration remains ordered.
func (c *worldgenCollector) readRegionPrograms(sources []WorldgenSource) error {
	if !worldgenHasMod(c.snapshot, "terrablender") {
		return nil
	}
	paths := make(map[string]string, len(sources))
	for _, source := range sources {
		paths[source.Name] = source.Path
	}
	type inspection struct {
		discovery modjar.Discovery
		err       error
	}
	results := make([]inspection, len(c.snapshot.Mods))
	stage := seedMapStage("regions.discover")
	defer stage("")
	jobs := make(chan int)
	var workers sync.WaitGroup
	for range min(max(4, runtime.NumCPU()), len(results)) {
		workers.Go(func() {
			for i := range jobs {
				mod := c.snapshot.Mods[i]
				if mod.ID == "terrablender" {
					if _, err := os.Stat(paths[mod.Source]); err != nil {
						results[i].err = err
					}
					continue
				}
				if r, ok := c.prefetch.discovery(paths[mod.Source]); ok {
					results[i].discovery, results[i].err = r.d, r.err
					continue
				}
				results[i].discovery, results[i].err = discoverTerraBlenderCached(c.ctx, paths[mod.Source])
			}
		})
	}
	for i := range c.snapshot.Mods {
		jobs <- i
	}
	close(jobs)
	workers.Wait()
	if err := c.ctx.Err(); err != nil {
		return err
	}
	mods := make(map[string]string, len(c.snapshot.Mods))
	for _, mod := range c.snapshot.Mods {
		mods[mod.ID] = mod.Version
	}
	hasRegions := make([]bool, len(results))
	for i, item := range results {
		hasRegions[i] = item.err == nil && item.discovery.HasRegions()
	}
	stage("regions.relevance")
	c.registrationSources = worldgenRegistrationSources(c.ctx, paths, c.snapshot.Mods, hasRegions)
	stage("regions.register")
	opened := make(map[string]*zip.ReadCloser)
	defer func() {
		for _, z := range opened {
			z.Close()
		}
	}()
	// Construct once, on demand. Dependencies inside loader-declared JARs are
	// part of the same classpath; arbitrary ZIP entries are not executable input.
	var classPath *modjar.Classpath
	var closeClasspath func()
	defer func() {
		if classPath != nil {
			classPath.Close()
		}
		if closeClasspath != nil {
			closeClasspath()
		}
	}()
	var classPathErr error
	loader, loaderErr := c.selectedLoader()
	var classPathOnce sync.Once
	ensureClassPath := func() error {
		classPathOnce.Do(func() {
			cpStage := seedMapStage("classpath.build")
			defer cpStage("")
			if loaderErr != nil {
				classPathErr = loaderErr
				return
			}
			var roots []modjar.Archive
			roots, closeClasspath, classPathErr = c.classArchives(sources)
			if classPathErr != nil {
				return
			}
			classPath, classPathErr = modjar.NewClasspath(c.ctx, roots, loader)
		})
		return classPathErr
	}
	readClass := func(name string) ([]byte, error) {
		if err := ensureClassPath(); err != nil {
			return nil, err
		}
		return classPath.Class(name)
	}
	readResources := func(name string) ([][]byte, error) {
		if err := ensureClassPath(); err != nil {
			return nil, err
		}
		return classPath.Resources(name)
	}
	reflectionOpen := func(name string) (bool, error) {
		if err := ensureClassPath(); err != nil {
			return false, err
		}
		return classPath.ReflectionOpen(name)
	}
	// runOne is the original per-mod registration step. config is the
	// configuration reader the interpreter sees.
	type outcome struct {
		program      WorldgenRegionProgram
		terraBlender bool
	}
	runOne := func(i int, config func(string) ([]byte, error)) outcome {
		mod := c.snapshot.Mods[i]
		inspection := results[i]
		tb := mod.TerraBlender
		if inspection.err == nil {
			tb = inspection.discovery.HasRegions()
		}
		program := WorldgenRegionProgram{ModID: mod.ID, Source: mod.Source, Discovery: inspection.discovery}
		if inspection.err != nil {
			program.Error = inspection.err.Error()
			return outcome{program, tb}
		}
		z, err := zip.OpenReader(paths[mod.Source])
		if err == nil {
			options := modjar.Options{Minecraft: c.snapshot.Minecraft, Symbols: c.symbols, JavaMajor: c.snapshot.JavaMajor, Loader: loader, LoaderVersion: c.snapshot.LoaderVersion, ModID: mod.ID, Mods: mods, Side: "DEDICATED_SERVER", Language: hostJavaLanguage(), Config: config, Class: readClass, Resources: readResources, ReflectionOpen: reflectionOpen, MixinArchives: func() ([]modjar.Archive, error) {
				if err := ensureClassPath(); err != nil {
					return nil, err
				}
				return classPath.Archives(), nil
			}}
			var result modjar.Result
			err = loaderErr
			if err == nil {
				seedMapProgress(c.ctx, "register", mod.ID)
				modStage := seedMapStage("register " + mod.ID)
				result, err = modjar.ReadTerraBlender(c.ctx, &z.Reader, options)
				if err == nil {
					sets, e := modjar.ReadSurfaceRules(c.ctx, &z.Reader, options)
					if e != nil {
						result.SurfaceRulesError = e.Error()
					}
					result.SurfaceRules = sets
				}
				modStage("")
			}
			z.Close()
			if err == nil {
				program.Result = &result
				// Discovery identifies possible registrations. An executed,
				// disabled integration contributes no regions to the engine.
				tb = len(result.Regions) != 0
			}
		}
		if err != nil {
			program.Error = fmt.Sprintf("%s: %v", mod.Loader, err)
		}
		return outcome{program, tb}
	}
	var jobs2 []int
	for i, inspection := range results {
		mod := &c.snapshot.Mods[i]
		if mod.ID == "terrablender" {
			continue
		}
		// A dependency alone does not make a biome provider. Conversely, an
		// optional integration need not declare TerraBlender as a dependency.
		if inspection.err == nil {
			mod.TerraBlender = inspection.discovery.HasRegions()
		}
		if mod.TerraBlender {
			jobs2 = append(jobs2, i)
		}
	}
	sequential := func() {
		for _, i := range jobs2 {
			o := runOne(i, c.registrationConfig)
			c.snapshot.Mods[i].TerraBlender = o.terraBlender
			c.snapshot.RegionPrograms = append(c.snapshot.RegionPrograms, o.program)
		}
	}
	if len(jobs2) < 2 || c.registrationConfigParallelOff {
		sequential()
		return c.ctx.Err()
	}
	// Registrations are independent interpreter runs over a read-only
	// classpath, so run them in parallel. Each run reads configuration through
	// a private log; the logs are replayed in mod order afterwards. If the
	// replay could differ from a sequential run (an earlier mod's failed read
	// would be cached as unreadable, or the 128-file cap would trip), the
	// parallel results are discarded and the sequential path runs instead.
	base := maps.Clone(c.snapshot.ModConfigs)
	logs := make([]*configLog, len(jobs2))
	outs := make([]outcome, len(jobs2))
	var wg sync.WaitGroup
	sem := make(chan struct{}, max(2, runtime.NumCPU()))
	for k, i := range jobs2 {
		logs[k] = &configLog{base: base, read: c.readConfig}
		wg.Go(func() {
			sem <- struct{}{}
			defer func() { <-sem }()
			outs[k] = runOne(i, logs[k].get)
		})
	}
	wg.Wait()
	if err := c.ctx.Err(); err != nil {
		return err
	}
	merged := maps.Clone(c.snapshot.ModConfigs)
	if merged == nil {
		merged = map[string][]byte{}
	}
	ok := true
	for _, l := range logs {
		for _, name := range l.names {
			if _, exists := merged[name]; exists {
				if merged[name] == nil {
					ok = false
				}
				continue
			}
			if len(merged) >= 128 {
				ok = false
				break
			}
			merged[name] = l.vals[name]
		}
	}
	if !ok {
		sequential()
		return c.ctx.Err()
	}
	if len(merged) > 0 {
		c.snapshot.ModConfigs = merged
	}
	for k, i := range jobs2 {
		c.snapshot.Mods[i].TerraBlender = outs[k].terraBlender
		c.snapshot.RegionPrograms = append(c.snapshot.RegionPrograms, outs[k].program)
	}
	return c.ctx.Err()
}

// configLog mirrors worldgenCollector.registrationConfig for one parallel
// registration without mutating the shared snapshot.
type configLog struct {
	mu    sync.Mutex
	base  map[string][]byte
	read  func(string) ([]byte, error)
	names []string
	vals  map[string][]byte
}

func (l *configLog) get(name string) ([]byte, error) {
	if data, exists := l.base[name]; exists {
		if data == nil {
			return nil, fmt.Errorf("unreadable configuration %s", name)
		}
		return data, nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if data, exists := l.vals[name]; exists {
		if data == nil {
			return nil, fmt.Errorf("unreadable configuration %s", name)
		}
		return data, nil
	}
	if l.read == nil {
		return nil, nil
	}
	data, err := l.read(name)
	if err == nil && len(data) > worldgenModConfigLimit {
		err = errors.New("configuration too large")
	}
	if data == nil && err == nil {
		return nil, nil
	}
	if l.vals == nil {
		l.vals = map[string][]byte{}
	}
	l.names = append(l.names, name)
	if err != nil {
		l.vals[name] = nil
		return nil, err
	}
	l.vals[name] = append([]byte{}, data...)
	return data, nil
}
