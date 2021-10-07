package pipeline

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/blakerouse/elastic-agent-sdk/pkg/component"
)

type Component struct {
	Executable string
	Info component.Info
}

func LoadComponents(path string) ([]*Component, error) {
	var comps []*Component
	files, err := ioutil.ReadDir(path)
	if err != nil {
		return nil, err
	}
	for _, file := range files {
		if file.IsDir() || !isExecutable(file.Mode()) {
			continue
		}
		out, err := exec.Command(filepath.Join(path, file.Name()), "info").Output()
		if err != nil {
			return nil, fmt.Errorf("failed to execute info for %s: %w", file.Name(), err)
		}
		var info component.Info
		err = json.Unmarshal(out, &info)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal info json for %s: %w", file.Name(), err)
		}
		comps = append(comps, &Component{
			Executable: filepath.Join(path, file.Name()),
			Info:       info,
		})
	}
	return comps, err
}

func isExecutable(mode os.FileMode) bool {
	return mode&0111 != 0
}
