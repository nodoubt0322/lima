// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package usrlocalsharelima

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"

	"github.com/lima-vm/lima/pkg/debugutil"
	"github.com/lima-vm/lima/pkg/limayaml"
	"github.com/sirupsen/logrus"
)

func Dir() (string, error) {
	self, err := os.Executable()
	if err != nil {
		return "", err
	}
	selfSt, err := os.Stat(self)
	if err != nil {
		return "", err
	}
	if selfSt.Mode()&fs.ModeSymlink != 0 {
		self, err = os.Readlink(self)
		if err != nil {
			return "", err
		}
	}

	ostype := limayaml.NewOS("linux")
	arch := limayaml.NewArch(runtime.GOARCH)
	if arch == "" {
		return "", fmt.Errorf("failed to get arch for %q", runtime.GOARCH)
	}

	// self:  /usr/local/bin/limactl
	selfDir := filepath.Dir(self)
	selfDirDir := filepath.Dir(selfDir)
	gaCandidates := []string{
		// candidate 0:
		// - self:  /Applications/Lima.app/Contents/MacOS/limactl
		// - agent: /Applications/Lima.app/Contents/MacOS/lima-guestagent.Linux-x86_64
		// - dir:   /Applications/Lima.app/Contents/MacOS
		filepath.Join(selfDir, "lima-guestagent."+ostype+"-"+arch),
		// candidate 1:
		// - self:  /usr/local/bin/limactl
		// - agent: /usr/local/share/lima/lima-guestagent.Linux-x86_64
		// - dir:   /usr/local/share/lima
		filepath.Join(selfDirDir, "share/lima/lima-guestagent."+ostype+"-"+arch),
		// TODO: support custom path
	}
	if debugutil.Debug {
		// candidate 2: launched by `~/go/bin/dlv dap`
		// - self: ${workspaceFolder}/cmd/limactl/__debug_bin_XXXXXX
		// - agent: ${workspaceFolder}/_output/share/lima/lima-guestagent.Linux-x86_64
		// - dir:  ${workspaceFolder}/_output/share/lima
		candidateForDebugBuild := filepath.Join(filepath.Dir(selfDirDir), "_output/share/lima/lima-guestagent."+ostype+"-"+arch)
		gaCandidates = append(gaCandidates, candidateForDebugBuild)
		logrus.Infof("debug mode detected, adding more guest agent candidates: %v", candidateForDebugBuild)
	}
	for _, gaCandidate := range gaCandidates {
		if _, err := os.Stat(gaCandidate); err == nil {
			return filepath.Dir(gaCandidate), nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		if _, err := os.Stat(gaCandidate + ".gz"); err == nil {
			return filepath.Dir(gaCandidate), nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
	}

	return "", fmt.Errorf("failed to find \"lima-guestagent.%s-%s\" binary for %q, attempted %v",
		ostype, arch, self, gaCandidates)
}

// GuestAgentBinary returns the guest agent binary, possibly with ".gz" suffix.
func GuestAgentBinary(ostype limayaml.OS, arch limayaml.Arch) (string, error) {
	if ostype == "" {
		return "", errors.New("os must be set")
	}
	if arch == "" {
		return "", errors.New("arch must be set")
	}
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	uncomp := filepath.Join(dir, "lima-guestagent."+ostype+"-"+arch)
	comp := uncomp + ".gz"
	res, err := chooseGABinary([]string{comp, uncomp})
	if err != nil {
		logrus.Debug(err)
		return "", fmt.Errorf("guest agent binary could not be found for %s-%s (Hint: try installing `lima-additional-guestagents` package)", ostype, arch)
	}
	return res, nil
}

func chooseGABinary(candidates []string) (string, error) {
	var entries []string
	for _, f := range candidates {
		if _, err := os.Stat(f); err != nil {
			if !errors.Is(err, fs.ErrNotExist) {
				logrus.WithError(err).Warnf("failed to stat %q", f)
			}
			continue
		}
		entries = append(entries, f)
	}
	switch len(entries) {
	case 0:
		return "", fmt.Errorf("%w: attempted %v", fs.ErrNotExist, candidates)
	case 1:
		return entries[0], nil
	default:
		logrus.Warnf("multiple files found, choosing %q from %v; consider removing the other ones",
			entries[0], candidates)
		return entries[0], nil
	}
}
