// SPDX-License-Identifier: Apache-2.0

// +build ignore

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"golang.org/x/sync/errgroup"
)

func main() {
	if err := run(); err != nil {
		panic(err)
	}
}

func run() error {
	out := "ebiten"
	_ = os.Remove(out)

	cachedir := ".cache"
	if err := os.MkdirAll(cachedir, 0755); err != nil {
		return err
	}

	srcs, err := filepath.Glob("*.cpp")
	if err != nil {
		return err
	}
	autogensrcs, err := filepath.Glob("autogen/*.cpp")
	if err != nil {
		return err
	}
	srcs = append(srcs, autogensrcs...)

	var objs []string
	var objsm sync.Mutex
	var g errgroup.Group
	sem := make(chan struct{}, runtime.NumCPU())
	for _, src := range srcs {
		src := src
		g.Go(func() error {
			sem <- struct{}{}
			defer func() {
				<-sem
			}()

			ppcmd := exec.Command("clang++", "-E", "-std=c++14", src)
			ppcmd.Stderr = os.Stderr
			hash := sha256.New()
			ppcmd.Stdout = hash
			if err := ppcmd.Run(); err != nil {
				return err
			}
			hashsum := hash.Sum(nil)
			obj := filepath.Join(cachedir, hex.EncodeToString(hashsum[:])+".o")
			objsm.Lock()
			objs = append(objs, obj)
			objsm.Unlock()
			if _, err := os.Stat(obj); err == nil {
				return nil
			} else if !os.IsNotExist(err) {
				return err
			}

			args := []string{"clang++",
				"-O3",
				"-Wall",
				"-std=c++14",
				"-pthread",
				"-I.",
				"-g",
				"-c",
				"-o", obj, src}
			fmt.Println(strings.Join(args, " "))
			buildcmd := exec.Command(args[0], args[1:]...)
			buildcmd.Stderr = os.Stderr
			if err := buildcmd.Run(); err != nil {
				return err
			}
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return err
	}

	args := []string{"clang++",
		"-Wall",
		"-std=c++14",
		"-framework", "OpenGL",
		"-lglfw",
		"-DGL_SILENCE_DEPRECATION",
		"-pthread",
		"-g",
		"-o", out}
	for _, obj := range objs {
		args = append(args, obj)
	}
	fmt.Println(strings.Join(args, " "))
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}

	return nil
}
