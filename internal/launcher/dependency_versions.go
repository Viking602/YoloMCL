package launcher

import (
	"context"
	"errors"
	"fmt"

	"github.com/Viking602/YoloMCL/internal/i18n"
)

const maxDependencyProjects = 128

var errDependencyLimit = errors.New(i18n.Text(false, "dependencies.limit"))

// dependencyVersion resolves an exact file or the first compatible release.
// Both preview and installation validate pinned files, which bypass the
// provider's game-version/loader filters.
func (s *Service) dependencyVersion(ctx context.Context, source string, d Dependency, mc, loader string) (ProjectVersion, error) {
	var v ProjectVersion
	var err error
	if d.VersionID != "" {
		v, err = s.projectVersion(ctx, source, d.ProjectID, d.VersionID)
	} else if d.ProjectID != "" {
		var versions []ProjectVersion
		versions, err = s.Versions(ctx, source, d.ProjectID, mc, loader)
		err = firstCompatible(versions, mc, loader, &v, err)
	} else {
		err = errors.New(i18n.Text(false, "dependencies.missing_identity"))
	}
	if err != nil {
		return v, err
	}
	if d.VersionID != "" && v.ID != d.VersionID {
		return v, errors.New(i18n.Text(false, "dependencies.project_mismatch"))
	}
	return v, checkDependencyVersion(v, source, d.ProjectID, mc, loader)
}

func checkDependencyVersion(v ProjectVersion, source, project, mc, loader string) error {
	if v.ID == "" || v.ProjectID == "" || v.Source != source || (project != "" && v.ProjectID != project) {
		return errors.New(i18n.Text(false, "dependencies.project_mismatch"))
	}
	if !compatible(v, mc, loader) {
		return fmt.Errorf(i18n.Text(false, "dependencies.incompatible"), v.Name, mc, loader)
	}
	return nil
}

// Read the exact installed file's dependencies, never those of a newer release.
// Imported jars without a platform record can still be identified by their
// hash/fingerprint. A failed lookup must not masquerade as a complete graph.
func (s *Service) installedDependencyVersion(ctx context.Context, r Root, source, project string, m Mod, mc, loader string) (ProjectVersion, error) {
	if m.Source == source && m.ProjectID == project && m.VersionID != "" {
		return s.dependencyVersion(ctx, source, Dependency{ProjectID: project, VersionID: m.VersionID}, mc, loader)
	}
	var v ProjectVersion
	var err error
	if source == "curseforge" {
		var path string
		path, err = safeJoin(r.VersionDir(), "mods/"+m.File)
		if err == nil {
			v, err = s.lookupCursePack(ctx, path)
		}
	} else {
		v, err = s.lookupMrpack(ctx, m.SHA1)
	}
	if err != nil {
		return v, err
	}
	// A mod-ID match can be provided by a bundled library in another project.
	// The hash/fingerprint identifies that containing jar: follow its actual
	// dependencies while keeping the requested project satisfied by the file.
	return v, checkDependencyVersion(v, source, v.ProjectID, mc, loader)
}
