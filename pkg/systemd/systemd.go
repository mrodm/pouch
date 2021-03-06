/*
Copyright 2017 Tuenti Technologies S.L. All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package systemd

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/coreos/go-systemd/daemon"
	"github.com/coreos/go-systemd/dbus"
	"github.com/coreos/go-systemd/util"
)

type SystemD interface {
	IsAvailable() bool
	CanNotify() bool
	Close()

	NotifyReady() error
	Reload(context.Context, string) error
}

type SystemdConfigurer interface {
	Enabled() bool
}

func New(c SystemdConfigurer) SystemD {
	return &systemd{
		enabled: c.Enabled(),
	}
}

type systemd struct {
	enabled bool

	name string
}

func (s *systemd) Close() {
}

func (s *systemd) IsAvailable() bool {
	if !s.enabled {
		return false
	}

	if !util.IsRunningSystemd() {
		log.Printf("Systemd is not running")
		return false
	}

	log.Println("systemd seems to be available")

	return true
}

const NotifySocketVar = "NOTIFY_SOCKET"

func (s *systemd) CanNotify() bool {
	notifySocket := os.Getenv(NotifySocketVar)
	if notifySocket == "" {
		log.Println("NOTIFY_SOCKET environment variable is not set")
		return false
	}

	if _, err := os.Stat(notifySocket); os.IsNotExist(err) {
		log.Printf("Notify socket (%s) doesn't exist\n", notifySocket)
	}
	return true
}

func (s *systemd) NotifyReady() error {
	sent, err := daemon.SdNotify(false, "READY=1")
	if err != nil {
		return fmt.Errorf("couldn't notify ready: %v", err)
	}
	if !sent {
		return fmt.Errorf("ready notification to systemd was not sent")
	}
	return nil
}

func (s *systemd) Reload(ctx context.Context, name string) error {
	c, err := dbus.New()
	if err != nil {
		return err
	}
	defer c.Close()

	result := make(chan string, 1)
	_, err = c.ReloadOrRestartUnit(name, "replace", result)
	if err != nil {
		return err
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case r := <-result:
		if r != "done" {
			return fmt.Errorf("reload job for %s is not done (found: %s)", name, r)
		}
	}
	return nil
}
