package launcher

import (
	"os/exec"

	"github.com/Viking602/YoloMCL/internal/launcher/memory"
	"github.com/Viking602/YoloMCL/internal/launcher/proctree"
	"github.com/Viking602/YoloMCL/internal/launcher/textenc"
)

type MemoryInfo = memory.MemoryInfo

type procTree = proctree.Tree

func DecodeLine(b []byte) string { return textenc.DecodeLine(b) }

func SystemMemory() (MemoryInfo, error) { return memory.SystemMemory() }

func prepareTree(c *exec.Cmd)                   { proctree.Prepare(c) }
func attachTree(c *exec.Cmd) (*procTree, error) { return proctree.Attach(c) }
func hideWindow(c *exec.Cmd)                    { proctree.HideWindow(c) }
func hideConsole(c *exec.Cmd)                   { proctree.HideConsole(c) }
