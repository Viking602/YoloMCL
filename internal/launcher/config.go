package launcher

import (
	"errors"
	"maps"
	"path/filepath"
	"slices"
)

func (r Root) clone() Root {
	r.VersionConfigs = maps.Clone(r.VersionConfigs)
	return r
}

func cloneRoots(roots []Root) []Root {
	out := slices.Clone(roots)
	for i := range out {
		out[i] = out[i].clone()
	}
	return out
}

// clone creates an isolated draft, including nested version override maps.
func (c config) clone() config {
	c.Roots = cloneRoots(c.Roots)
	c.Accounts = slices.Clone(c.Accounts)
	c.AuthServers = slices.Clone(c.AuthServers)
	return c
}

// saveConfigLocked publishes a draft only after it has reached disk. The
// caller must hold s.mu and must not mutate the draft after committing it.
func (s *Service) saveConfigLocked(next config) error {
	if err := atomicJSON(filepath.Join(s.dir, "config.json"), next); err != nil {
		return err
	}
	s.cfg = next
	return nil
}

// setRootVersion persists an install's result without changing the user's
// active instance when a background job finishes.
func (s *Service) setRootVersion(rootID, version string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	draft := s.cfg.clone()
	for i := range draft.Roots {
		if draft.Roots[i].ID == rootID {
			draft.Roots[i].SelectedVersion = version
			return s.saveConfigLocked(draft)
		}
	}
	return errors.New("目录不存在")
}
