package modjar

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"runtime"
	"slices"
	"strings"
	"sync"
)

// Discovery identifies registration code, not a claim that its branch runs.
// Loader execution must still apply configuration and optional dependencies.
// Ordinary Java classes, mixins, and mere TerraBlender dependency declarations
// do not establish a region registration.
type Discovery struct {
	RegionClasses []string `json:",omitempty"`
	Callers       []string `json:",omitempty"`
}

func (d Discovery) HasRegions() bool { return len(d.RegionClasses)+len(d.Callers) != 0 }

// DiscoverTerraBlender recognizes the framework's actual superclass and method
// references. It is independent of mod identity and Minecraft release names.
func DiscoverTerraBlender(ctx context.Context, z *zip.Reader) (result Discovery, err error) {
	// Collect candidates in archive order and find where the byte budget trips;
	// entries after that point are never read, exactly like a sequential scan.
	var entries []*zip.File
	budgetHit := false
	var total uint64
	for _, entry := range z.File {
		if !strings.HasSuffix(entry.Name, ".class") || strings.HasPrefix(entry.Name, "META-INF/") {
			continue
		}
		total += entry.UncompressedSize64
		if entry.UncompressedSize64 > 2<<20 || total > 256<<20 {
			budgetHit = true
			break
		}
		entries = append(entries, entry)
	}
	type outcome struct {
		err     error
		panic   any
		regions []string
		callers []string
	}
	outcomes := make([]outcome, len(entries))
	scan := func(i int) {
		o := &outcomes[i]
		defer func() {
			if p := recover(); p != nil {
				o.panic = p
			}
		}()
		entry := entries[i]
		stream, e := entry.Open()
		if e != nil {
			o.err = e
			return
		}
		data, e := io.ReadAll(io.LimitReader(stream, (2<<20)+1))
		stream.Close()
		if e != nil {
			o.err = e
			return
		}
		if len(data) > 2<<20 {
			o.err = fmt.Errorf("registration class size limit")
			return
		}
		// This is only a prefilter: strings and operands never establish usage.
		if !bytes.Contains(data, []byte("terrablender/api/Region")) {
			return
		}
		c := parseClass(data)
		if c.name+".class" != entry.Name {
			fail("class entry/name mismatch")
		}
		if c.super == "terrablender/api/Region" {
			o.regions = append(o.regions, c.name)
		}
		for _, m := range c.methods {
			for _, r := range methodReferences(c, m) {
				if r.owner == "terrablender/api/Regions" && r.name == "register" {
					o.callers = append(o.callers, c.name+"."+m.name+m.desc)
					break
				}
			}
		}
	}
	// Archive entries are independent ReaderAt sections, so inflate them in
	// parallel. Outcomes are folded in archive order below so the first error
	// wins just as in a sequential scan.
	workers := min(runtime.NumCPU(), max(1, len(entries)/64))
	next := make(chan int)
	var wg sync.WaitGroup
	for range workers {
		wg.Go(func() {
			for i := range next {
				if ctx.Err() == nil {
					scan(i)
				}
			}
		})
	}
	for i := range entries {
		next <- i
	}
	close(next)
	wg.Wait()
	for _, o := range outcomes {
		if err = ctx.Err(); err != nil {
			return Discovery{}, err
		}
		if o.panic != nil {
			return Discovery{}, fmt.Errorf("registration discovery: %v", o.panic)
		}
		if o.err != nil {
			return result, o.err
		}
		result.RegionClasses = append(result.RegionClasses, o.regions...)
		result.Callers = append(result.Callers, o.callers...)
	}
	if err = ctx.Err(); err != nil {
		return Discovery{}, err
	}
	if budgetHit {
		return result, fmt.Errorf("registration discovery byte budget exceeded")
	}
	slices.Sort(result.RegionClasses)
	slices.Sort(result.Callers)
	return result, nil
}
