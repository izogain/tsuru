// Copyright 2016 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package provisiontest

import (
	"bytes"
	"errors"
	"io/ioutil"
	"sort"
	"testing"

	"github.com/tsuru/config"
	"github.com/tsuru/tsuru/app"
	"github.com/tsuru/tsuru/app/bind"
	"github.com/tsuru/tsuru/db"
	"github.com/tsuru/tsuru/db/dbtest"
	"github.com/tsuru/tsuru/event"
	"github.com/tsuru/tsuru/permission"
	"github.com/tsuru/tsuru/provision"
	"github.com/tsuru/tsuru/quota"
	"github.com/tsuru/tsuru/router/routertest"
	"gopkg.in/check.v1"
	"gopkg.in/mgo.v2/bson"
)

func Test(t *testing.T) {
	check.TestingT(t)
}

type S struct{}

var _ = check.Suite(&S{})

func (s *S) SetUpSuite(c *check.C) {
	config.Set("database:url", "127.0.0.1:27017")
	config.Set("database:name", "fake_provision_tests_s")
}

func (s *S) SetUpTest(c *check.C) {
	conn, err := db.Conn()
	c.Assert(err, check.IsNil)
	defer conn.Close()
	err = dbtest.ClearAllCollections(conn.Apps().Database)
	c.Assert(err, check.IsNil)
	routertest.FakeRouter.Reset()
}

func (s *S) TestFakeAppAddUnit(c *check.C) {
	app := NewFakeApp("jean", "mj", 0)
	app.AddUnit(provision.Unit{ID: "jean-0"})
	c.Assert(app.units, check.HasLen, 1)
}

func (s *S) TestFakeAppGetMemory(c *check.C) {
	app := NewFakeApp("sou", "otm", 0)
	app.Memory = 100
	c.Assert(app.GetMemory(), check.Equals, int64(100))
}

func (s *S) TestFakeAppGetSwap(c *check.C) {
	app := NewFakeApp("sou", "otm", 0)
	c.Assert(app.GetSwap(), check.Equals, int64(0))
}

func (s *S) TestFakeAppSerializeEnvVars(c *check.C) {
	app := NewFakeApp("sou", "otm", 0)
	err := app.SerializeEnvVars()
	c.Assert(err, check.IsNil)
	c.Assert(app.Commands, check.DeepEquals, []string{"serialize"})
}

func (s *S) TestEnvs(c *check.C) {
	app := FakeApp{name: "time"}
	env := bind.EnvVar{
		Name:   "http_proxy",
		Value:  "http://theirproxy.com:3128/",
		Public: true,
	}
	app.SetEnv(env)
	envs := map[string]bind.EnvVar{
		"http_proxy": {
			Name:   "http_proxy",
			Value:  "http://theirproxy.com:3128/",
			Public: true,
		},
	}
	c.Assert(envs, check.DeepEquals, app.env)
}

func (s *S) TestSetEnvs(c *check.C) {
	app := FakeApp{name: "time"}
	envs := []bind.EnvVar{
		{
			Name:   "http_proxy",
			Value:  "http://theirproxy.com:3128/",
			Public: true,
		},
		{
			Name:   "https_proxy",
			Value:  "http://theirproxy.com:3128/",
			Public: true,
		},
	}
	app.SetEnvs(
		bind.SetEnvApp{
			Envs:          envs,
			PublicOnly:    false,
			ShouldRestart: true,
		}, nil)
	expected := map[string]bind.EnvVar{
		"http_proxy": {
			Name:   "http_proxy",
			Value:  "http://theirproxy.com:3128/",
			Public: true,
		},
		"https_proxy": {
			Name:   "https_proxy",
			Value:  "http://theirproxy.com:3128/",
			Public: true,
		},
	}
	c.Assert(app.env, check.DeepEquals, expected)
}

func (s *S) TestGetUnitsReturnUnits(c *check.C) {
	a := NewFakeApp("foo", "static", 2)
	units, err := a.GetUnits()
	c.Assert(err, check.IsNil)
	c.Assert(units, check.HasLen, 2)
	c.Assert(a.units, check.HasLen, 2)
	c.Assert(units[0].GetID(), check.Equals, a.units[0].ID)
	c.Assert(units[1].GetID(), check.Equals, a.units[1].ID)
}

func (s *S) TestUnsetEnvs(c *check.C) {
	app := FakeApp{name: "time"}
	env := bind.EnvVar{
		Name:   "http_proxy",
		Value:  "http://theirproxy.com:3128/",
		Public: true,
	}
	app.SetEnv(env)
	app.UnsetEnvs(
		bind.UnsetEnvApp{
			VariableNames: []string{"http_proxy"},
			PublicOnly:    false,
			ShouldRestart: true,
		}, nil)
	c.Assert(app.env, check.DeepEquals, map[string]bind.EnvVar{})
}

func (s *S) TestFakeAppBindUnit(c *check.C) {
	var unit provision.Unit
	app := NewFakeApp("sou", "otm", 0)
	err := app.BindUnit(&unit)
	c.Assert(err, check.IsNil)
	c.Assert(app.HasBind(&unit), check.Equals, true)
}

func (s *S) TestFakeAppUnbindUnit(c *check.C) {
	var unit provision.Unit
	app := NewFakeApp("sou", "otm", 0)
	err := app.BindUnit(&unit)
	c.Assert(err, check.IsNil)
	err = app.UnbindUnit(&unit)
	c.Assert(err, check.IsNil)
	c.Assert(app.HasBind(&unit), check.Equals, false)
}

func (s *S) TestFakeAppUnbindUnitNotBound(c *check.C) {
	var unit provision.Unit
	app := NewFakeApp("sou", "otm", 0)
	err := app.UnbindUnit(&unit)
	c.Assert(err, check.NotNil)
	c.Assert(err.Error(), check.Equals, "not bound")
}

func (s *S) TestFakeAppGetQuota(c *check.C) {
	app := NewFakeApp("sou", "otm", 0)
	c.Assert(app.GetQuota(), check.DeepEquals, quota.Unlimited)
	q := quota.Quota{Limit: 10, InUse: 3}
	app.Quota = q
	c.Assert(app.GetQuota(), check.DeepEquals, q)
}

func (s *S) TestFakeAppSetQuotaInUse(c *check.C) {
	q := quota.Quota{Limit: 10, InUse: 3}
	app := NewFakeApp("sou", "otm", 0)
	app.Quota = q
	c.Assert(app.GetQuota(), check.DeepEquals, q)
	q.InUse = 8
	err := app.SetQuotaInUse(q.InUse)
	c.Assert(err, check.IsNil)
	c.Assert(app.GetQuota(), check.DeepEquals, q)
	err = app.SetQuotaInUse(q.Limit + 1)
	c.Assert(err, check.NotNil)
	e, ok := err.(*quota.QuotaExceededError)
	c.Assert(ok, check.Equals, true)
	c.Assert(e.Available, check.Equals, uint(q.Limit))
	c.Assert(e.Requested, check.Equals, uint(q.Limit+1))
}

func (s *S) TestFakeAppGetCname(c *check.C) {
	app := NewFakeApp("sou", "otm", 0)
	app.cname = []string{"cname1", "cname2"}
	c.Assert(app.GetCname(), check.DeepEquals, []string{"cname1", "cname2"})
}

func (s *S) TestFakeAppGetInstances(c *check.C) {
	instance1 := bind.ServiceInstance{Name: "inst1"}
	instance2 := bind.ServiceInstance{Name: "inst2"}
	app := NewFakeApp("sou", "otm", 0)
	app.instances["mysql"] = []bind.ServiceInstance{instance1, instance2}
	instances := app.GetInstances("mysql")
	c.Assert(instances, check.DeepEquals, []bind.ServiceInstance{instance1, instance2})
	instances = app.GetInstances("mongodb")
	c.Assert(instances, check.HasLen, 0)
}

func (s *S) TestFakeAppAddInstance(c *check.C) {
	instance1 := bind.ServiceInstance{Name: "inst1"}
	instance2 := bind.ServiceInstance{Name: "inst2"}
	app := NewFakeApp("sou", "otm", 0)
	err := app.AddInstance(
		bind.InstanceApp{
			ServiceName:   "mysql",
			Instance:      instance1,
			ShouldRestart: true,
		}, nil)
	c.Assert(err, check.IsNil)
	err = app.AddInstance(
		bind.InstanceApp{
			ServiceName:   "mongodb",
			Instance:      instance2,
			ShouldRestart: false,
		}, nil)
	c.Assert(err, check.IsNil)
	instances := app.GetInstances("mysql")
	c.Assert(instances, check.DeepEquals, []bind.ServiceInstance{instance1})
	instances = app.GetInstances("mongodb")
	c.Assert(instances, check.DeepEquals, []bind.ServiceInstance{instance2})
	instances = app.GetInstances("redis")
	c.Assert(instances, check.HasLen, 0)
}

func (s *S) TestFakeAppRemoveInstance(c *check.C) {
	instance1 := bind.ServiceInstance{Name: "inst1"}
	instance2 := bind.ServiceInstance{Name: "inst2"}
	app := NewFakeApp("sou", "otm", 0)
	app.AddInstance(
		bind.InstanceApp{
			ServiceName:   "mysql",
			Instance:      instance1,
			ShouldRestart: true,
		}, nil)
	app.AddInstance(
		bind.InstanceApp{
			ServiceName:   "mongodb",
			Instance:      instance2,
			ShouldRestart: false,
		}, nil)
	err := app.RemoveInstance(
		bind.InstanceApp{
			ServiceName:   "mysql",
			Instance:      instance1,
			ShouldRestart: true,
		}, nil)
	c.Assert(err, check.IsNil)
	instances := app.GetInstances("mysql")
	c.Assert(instances, check.HasLen, 0)
	instances = app.GetInstances("mongodb")
	c.Assert(instances, check.HasLen, 1)
}

func (s *S) TestFakeAppRemoveInstanceNotFound(c *check.C) {
	instance1 := bind.ServiceInstance{Name: "inst1"}
	instance2 := bind.ServiceInstance{Name: "inst2"}
	app := NewFakeApp("sou", "otm", 0)
	app.AddInstance(
		bind.InstanceApp{
			ServiceName:   "mysql",
			Instance:      instance1,
			ShouldRestart: true,
		}, nil)
	err := app.RemoveInstance(
		bind.InstanceApp{
			ServiceName:   "mysql",
			Instance:      instance2,
			ShouldRestart: true,
		}, nil)
	c.Assert(err.Error(), check.Equals, "instance not found")
}

func (s *S) TestFakeAppRemoveInstanceServiceNotFound(c *check.C) {
	instance := bind.ServiceInstance{Name: "inst1"}
	app := NewFakeApp("sou", "otm", 0)
	err := app.RemoveInstance(
		bind.InstanceApp{
			ServiceName:   "mysql",
			Instance:      instance,
			ShouldRestart: true,
		}, nil)
	c.Assert(err.Error(), check.Equals, "instance not found")
}

func (s *S) TestFakeAppLogs(c *check.C) {
	app := NewFakeApp("sou", "otm", 0)
	app.Log("something happened", "[tsuru]", "[api]")
	c.Assert(app.Logs(), check.DeepEquals, []string{"[tsuru][api]something happened"})
}

func (s *S) TestFakeAppHasLog(c *check.C) {
	app := NewFakeApp("sou", "otm", 0)
	app.Log("something happened", "[tsuru]", "[api]")
	c.Assert(app.HasLog("[tsuru]", "[api]", "something happened"), check.Equals, true)
	c.Assert(app.HasLog("tsuru", "api", "something happened"), check.Equals, false)
}

func (s *S) TestProvisioned(c *check.C) {
	app := NewFakeApp("red-sector", "rush", 1)
	p := NewFakeProvisioner()
	err := p.Provision(app)
	c.Assert(err, check.IsNil)
	c.Assert(p.Provisioned(app), check.Equals, true)
	otherapp := NewFakeApp("blue-sector", "rush", 1)
	c.Assert(p.Provisioned(otherapp), check.Equals, false)
}

func (s *S) TestRestarts(c *check.C) {
	app1 := NewFakeApp("fairy-tale", "shaman", 1)
	app2 := NewFakeApp("unfairy-tale", "shaman", 1)
	p := NewFakeProvisioner()
	p.apps = map[string]provisionedApp{
		app1.GetName(): {app: app1, restarts: map[string]int{"": 10, "web": 2}},
		app2.GetName(): {app: app1, restarts: map[string]int{"": 0}},
	}
	c.Assert(p.Restarts(app1, ""), check.Equals, 10)
	c.Assert(p.Restarts(app1, "web"), check.Equals, 2)
	c.Assert(p.Restarts(app2, ""), check.Equals, 0)
	c.Assert(p.Restarts(NewFakeApp("pride", "shaman", 1), ""), check.Equals, 0)
}

func (s *S) TestStarts(c *check.C) {
	app1 := NewFakeApp("fairy-tale", "shaman", 1)
	app2 := NewFakeApp("unfairy-tale", "shaman", 1)
	p := NewFakeProvisioner()
	p.apps = map[string]provisionedApp{
		app1.GetName(): {app: app1, starts: map[string]int{"web": 10, "worker": 1}},
		app2.GetName(): {app: app1, starts: map[string]int{"": 0}},
	}
	c.Assert(p.Starts(app1, "web"), check.Equals, 10)
	c.Assert(p.Starts(app1, "worker"), check.Equals, 1)
	c.Assert(p.Starts(app2, ""), check.Equals, 0)
	c.Assert(p.Starts(NewFakeApp("pride", "shaman", 1), ""), check.Equals, 0)
}

func (s *S) TestStops(c *check.C) {
	app1 := NewFakeApp("fairy-tale", "shaman", 1)
	app2 := NewFakeApp("unfairy-tale", "shaman", 1)
	p := NewFakeProvisioner()
	p.apps = map[string]provisionedApp{
		app1.GetName(): {app: app1, stops: map[string]int{"web": 10, "worker": 1}},
		app2.GetName(): {app: app1, stops: map[string]int{"": 0}},
	}
	c.Assert(p.Stops(app1, "web"), check.Equals, 10)
	c.Assert(p.Stops(app1, "worker"), check.Equals, 1)
	c.Assert(p.Stops(app2, ""), check.Equals, 0)
	c.Assert(p.Stops(NewFakeApp("pride", "shaman", 1), ""), check.Equals, 0)
}

func (s *S) TestSleeps(c *check.C) {
	app1 := NewFakeApp("fairy-tale", "shaman", 1)
	app2 := NewFakeApp("unfairy-tale", "shaman", 1)
	p := NewFakeProvisioner()
	p.apps = map[string]provisionedApp{
		app1.GetName(): {app: app1, sleeps: map[string]int{"web": 10, "worker": 1}},
		app2.GetName(): {app: app1, sleeps: map[string]int{"": 0}},
	}
	c.Assert(p.Sleeps(app1, "web"), check.Equals, 10)
	c.Assert(p.Sleeps(app1, "worker"), check.Equals, 1)
	c.Assert(p.Sleeps(app2, ""), check.Equals, 0)
	c.Assert(p.Sleeps(NewFakeApp("pride", "shaman", 1), ""), check.Equals, 0)
}

func (s *S) TestGetCmds(c *check.C) {
	app := NewFakeApp("enemy-within", "rush", 1)
	p := NewFakeProvisioner()
	p.cmds = []Cmd{
		{Cmd: "ls -lh", App: app},
		{Cmd: "ls -lah", App: app},
	}
	c.Assert(p.GetCmds("ls -lh", app), check.HasLen, 1)
	c.Assert(p.GetCmds("l", app), check.HasLen, 0)
	c.Assert(p.GetCmds("", app), check.HasLen, 2)
	otherapp := NewFakeApp("enemy-without", "rush", 1)
	c.Assert(p.GetCmds("ls -lh", otherapp), check.HasLen, 0)
	c.Assert(p.GetCmds("", otherapp), check.HasLen, 0)
}

func (s *S) TestGetUnits(c *check.C) {
	list := []provision.Unit{
		{ID: "chain-lighting-0", AppName: "chain-lighting", ProcessName: "web", Type: "django", Ip: "10.10.10.10", Status: provision.StatusStarted},
		{ID: "chain-lighting-1", AppName: "chain-lighting", ProcessName: "web", Type: "django", Ip: "10.10.10.15", Status: provision.StatusStarted},
	}
	app := NewFakeApp("chain-lighting", "rush", 1)
	p := NewFakeProvisioner()
	p.apps = map[string]provisionedApp{
		app.GetName(): {app: app, units: list},
	}
	units := p.GetUnits(app)
	c.Assert(units, check.DeepEquals, list)
}

func (s *S) TestPrepareOutput(c *check.C) {
	output := []byte("the body eletric")
	p := NewFakeProvisioner()
	p.PrepareOutput(output)
	got := <-p.outputs
	c.Assert(string(got), check.Equals, string(output))
}

func (s *S) TestPrepareFailure(c *check.C) {
	err := errors.New("the body eletric")
	p := NewFakeProvisioner()
	p.PrepareFailure("Rush", err)
	got := <-p.failures
	c.Assert(got.method, check.Equals, "Rush")
	c.Assert(got.err.Error(), check.Equals, "the body eletric")
}

func (s *S) TestArchiveDeploy(c *check.C) {
	app := NewFakeApp("soul", "arch", 1)
	p := NewFakeProvisioner()
	err := p.Provision(app)
	c.Assert(err, check.IsNil)
	evt, err := event.New(&event.Opts{
		Target:   event.Target{Type: "app", Value: app.name},
		Kind:     permission.PermAppDeploy,
		RawOwner: event.Owner{Type: event.OwnerTypeUser, Name: "me@me.com"},
		Allowed:  event.Allowed(permission.PermApp),
	})
	c.Assert(err, check.IsNil)
	_, err = p.ArchiveDeploy(app, "https://s3.amazonaws.com/smt/archive.tar.gz", evt)
	c.Assert(err, check.IsNil)
	err = evt.Done(nil)
	c.Assert(err, check.IsNil)
	c.Assert(evt.Log, check.Equals, "Archive deploy called")
	c.Assert(p.apps[app.GetName()].lastArchive, check.Equals, "https://s3.amazonaws.com/smt/archive.tar.gz")
}

func (s *S) TestArchiveDeployUnknownApp(c *check.C) {
	app := NewFakeApp("soul", "arch", 1)
	p := NewFakeProvisioner()
	evt, err := event.New(&event.Opts{
		Target:   event.Target{Type: "app", Value: app.name},
		Kind:     permission.PermAppDeploy,
		RawOwner: event.Owner{Type: event.OwnerTypeUser, Name: "me@me.com"},
		Allowed:  event.Allowed(permission.PermApp),
	})
	c.Assert(err, check.IsNil)
	_, err = p.ArchiveDeploy(app, "https://s3.amazonaws.com/smt/archive.tar.gz", evt)
	c.Assert(err, check.Equals, errNotProvisioned)
}

func (s *S) TestArchiveDeployWithPreparedFailure(c *check.C) {
	app := NewFakeApp("soul", "arch", 1)
	evt, err := event.New(&event.Opts{
		Target:   event.Target{Type: "app", Value: app.name},
		Kind:     permission.PermAppDeploy,
		RawOwner: event.Owner{Type: event.OwnerTypeUser, Name: "me@me.com"},
		Allowed:  event.Allowed(permission.PermApp),
	})
	c.Assert(err, check.IsNil)
	p := NewFakeProvisioner()
	err = p.Provision(app)
	c.Assert(err, check.IsNil)
	p.PrepareFailure("ArchiveDeploy", errors.New("not really"))
	_, err = p.ArchiveDeploy(app, "https://s3.amazonaws.com/smt/archive.tar.gz", evt)
	c.Assert(err, check.ErrorMatches, "not really")
}

func (s *S) TestUploadDeploy(c *check.C) {
	var input bytes.Buffer
	file := ioutil.NopCloser(&input)
	app := NewFakeApp("soul", "arch", 1)
	evt, err := event.New(&event.Opts{
		Target:   event.Target{Type: "app", Value: app.name},
		Kind:     permission.PermAppDeploy,
		RawOwner: event.Owner{Type: event.OwnerTypeUser, Name: "me@me.com"},
		Allowed:  event.Allowed(permission.PermApp),
	})
	c.Assert(err, check.IsNil)
	p := NewFakeProvisioner()
	err = p.Provision(app)
	c.Assert(err, check.IsNil)
	_, err = p.UploadDeploy(app, file, 0, false, evt)
	c.Assert(err, check.IsNil)
	err = evt.Done(nil)
	c.Assert(err, check.IsNil)
	c.Assert(evt.Log, check.Equals, "Upload deploy called")
	c.Assert(p.apps[app.GetName()].lastFile, check.Equals, file)
}

func (s *S) TestUploadDeployUnknownApp(c *check.C) {
	app := NewFakeApp("soul", "arch", 1)
	evt, err := event.New(&event.Opts{
		Target:   event.Target{Type: "app", Value: app.name},
		Kind:     permission.PermAppDeploy,
		RawOwner: event.Owner{Type: event.OwnerTypeUser, Name: "me@me.com"},
		Allowed:  event.Allowed(permission.PermApp),
	})
	c.Assert(err, check.IsNil)
	p := NewFakeProvisioner()
	_, err = p.UploadDeploy(app, nil, 0, false, evt)
	c.Assert(err, check.Equals, errNotProvisioned)
}

func (s *S) TestUploadDeployWithPreparedFailure(c *check.C) {
	app := NewFakeApp("soul", "arch", 1)
	evt, err := event.New(&event.Opts{
		Target:   event.Target{Type: "app", Value: app.name},
		Kind:     permission.PermAppDeploy,
		RawOwner: event.Owner{Type: event.OwnerTypeUser, Name: "me@me.com"},
		Allowed:  event.Allowed(permission.PermApp),
	})
	c.Assert(err, check.IsNil)
	p := NewFakeProvisioner()
	err = p.Provision(app)
	c.Assert(err, check.IsNil)
	p.PrepareFailure("UploadDeploy", errors.New("not really"))
	_, err = p.UploadDeploy(app, nil, 0, false, evt)
	c.Assert(err, check.ErrorMatches, "not really")
}

func (s *S) TestImageDeploy(c *check.C) {
	app := NewFakeApp("otherapp", "test", 1)
	evt, err := event.New(&event.Opts{
		Target:   event.Target{Type: "app", Value: app.name},
		Kind:     permission.PermAppDeploy,
		RawOwner: event.Owner{Type: event.OwnerTypeUser, Name: "me@me.com"},
		Allowed:  event.Allowed(permission.PermApp),
	})
	c.Assert(err, check.IsNil)
	p := NewFakeProvisioner()
	err = p.Provision(app)
	c.Assert(err, check.IsNil)
	_, err = p.ImageDeploy(app, "image/deploy", evt)
	c.Assert(err, check.IsNil)
	err = evt.Done(nil)
	c.Assert(err, check.IsNil)
	c.Assert(evt.Log, check.Equals, "Image deploy called")
	c.Assert(p.apps[app.GetName()].image, check.Equals, "image/deploy")
}

func (s *S) TestImageDeployWithPrepareFailure(c *check.C) {
	app := NewFakeApp("otherapp", "test", 1)
	evt, err := event.New(&event.Opts{
		Target:   event.Target{Type: "app", Value: app.name},
		Kind:     permission.PermAppDeploy,
		RawOwner: event.Owner{Type: event.OwnerTypeUser, Name: "me@me.com"},
		Allowed:  event.Allowed(permission.PermApp),
	})
	c.Assert(err, check.IsNil)
	p := NewFakeProvisioner()
	err = p.Provision(app)
	c.Assert(err, check.IsNil)
	p.PrepareFailure("ImageDeploy", errors.New("not really"))
	_, err = p.ImageDeploy(app, "", evt)
	c.Assert(err, check.ErrorMatches, "not really")
}

func (s *S) TestProvision(c *check.C) {
	app := NewFakeApp("kid-gloves", "rush", 1)
	p := NewFakeProvisioner()
	err := p.Provision(app)
	c.Assert(err, check.IsNil)
	pApp := p.apps[app.GetName()]
	c.Assert(pApp.app, check.DeepEquals, app)
	c.Assert(pApp.units, check.HasLen, 0)
	c.Assert(routertest.FakeRouter.HasBackend(app.GetName()), check.Equals, true)
}

func (s *S) TestProvisionWithPreparedFailure(c *check.C) {
	app := NewFakeApp("kid-gloves", "rush", 1)
	p := NewFakeProvisioner()
	p.PrepareFailure("Provision", errors.New("Failed to provision."))
	err := p.Provision(app)
	c.Assert(err, check.NotNil)
	c.Assert(err.Error(), check.Equals, "Failed to provision.")
}

func (s *S) TestDoubleProvision(c *check.C) {
	app := NewFakeApp("kid-gloves", "rush", 1)
	p := NewFakeProvisioner()
	err := p.Provision(app)
	c.Assert(err, check.IsNil)
	err = p.Provision(app)
	c.Assert(err, check.NotNil)
	c.Assert(err.Error(), check.Equals, "App already provisioned.")
}

func (s *S) TestRestart(c *check.C) {
	conn, err := db.Conn()
	c.Assert(err, check.IsNil)
	defer conn.Close()
	a := NewFakeApp("kid-gloves", "rush", 1)
	nApp := app.App{
		Name: a.name,
	}
	err = conn.Apps().Insert(nApp)
	defer conn.Apps().Remove(bson.M{"name": nApp.Name})
	c.Assert(err, check.IsNil)
	p := NewFakeProvisioner()
	p.Provision(a)
	err = p.Restart(a, "web", nil)
	c.Assert(err, check.IsNil)
	c.Assert(p.Restarts(a, "web"), check.Equals, 1)
}

func (s *S) TestStart(c *check.C) {
	app := NewFakeApp("kid-gloves", "rush", 1)
	p := NewFakeProvisioner()
	p.Provision(app)
	err := p.Start(app, "")
	c.Assert(err, check.IsNil)
	err = p.Start(app, "web")
	c.Assert(err, check.IsNil)
	c.Assert(p.Starts(app, ""), check.Equals, 1)
	c.Assert(p.Starts(app, "web"), check.Equals, 1)
}

func (s *S) TestStop(c *check.C) {
	app := NewFakeApp("kid-gloves", "rush", 1)
	p := NewFakeProvisioner()
	p.Provision(app)
	err := p.Stop(app, "")
	c.Assert(err, check.IsNil)
	c.Assert(p.Stops(app, ""), check.Equals, 1)
}

func (s *S) TestSleep(c *check.C) {
	app := NewFakeApp("kid-gloves", "rush", 1)
	p := NewFakeProvisioner()
	p.Provision(app)
	err := p.Sleep(app, "")
	c.Assert(err, check.IsNil)
	c.Assert(p.Sleeps(app, ""), check.Equals, 1)
}

func (s *S) TestRestartNotProvisioned(c *check.C) {
	app := NewFakeApp("kid-gloves", "rush", 1)
	p := NewFakeProvisioner()
	err := p.Restart(app, "web", nil)
	c.Assert(err, check.Equals, errNotProvisioned)
}

func (s *S) TestRestartWithPreparedFailure(c *check.C) {
	app := NewFakeApp("fairy-tale", "shaman", 1)
	p := NewFakeProvisioner()
	p.PrepareFailure("Restart", errors.New("Failed to restart."))
	err := p.Restart(app, "web", nil)
	c.Assert(err, check.NotNil)
	c.Assert(err.Error(), check.Equals, "Failed to restart.")
}

func (s *S) TestDestroy(c *check.C) {
	app := NewFakeApp("kid-gloves", "rush", 1)
	p := NewFakeProvisioner()
	p.Provision(app)
	err := p.Destroy(app)
	c.Assert(err, check.IsNil)
	c.Assert(p.Provisioned(app), check.Equals, false)
}

func (s *S) TestDestroyWithPreparedFailure(c *check.C) {
	app := NewFakeApp("red-lenses", "rush", 1)
	p := NewFakeProvisioner()
	p.PrepareFailure("Destroy", errors.New("Failed to destroy."))
	err := p.Destroy(app)
	c.Assert(err, check.NotNil)
	c.Assert(err.Error(), check.Equals, "Failed to destroy.")
}

func (s *S) TestDestroyNotProvisionedApp(c *check.C) {
	app := NewFakeApp("red-lenses", "rush", 1)
	p := NewFakeProvisioner()
	err := p.Destroy(app)
	c.Assert(err, check.Equals, errNotProvisioned)
}

func (s *S) TestAddUnits(c *check.C) {
	app := NewFakeApp("mystic-rhythms", "rush", 0)
	p := NewFakeProvisioner()
	err := p.Provision(app)
	c.Assert(err, check.IsNil)
	_, err = p.AddUnits(app, 2, "web", nil)
	c.Assert(err, check.IsNil)
	_, err = p.AddUnits(app, 2, "worker", nil)
	c.Assert(err, check.IsNil)
	allUnits := p.GetUnits(app)
	c.Assert(allUnits, check.HasLen, 4)
	c.Assert(allUnits[0].ProcessName, check.Equals, "web")
	c.Assert(allUnits[1].ProcessName, check.Equals, "web")
	c.Assert(allUnits[2].ProcessName, check.Equals, "worker")
	c.Assert(allUnits[3].ProcessName, check.Equals, "worker")
	c.Assert(routertest.FakeRouter.HasRoute(app.GetName(), allUnits[0].Address.String()), check.Equals, true)
	c.Assert(routertest.FakeRouter.HasRoute(app.GetName(), allUnits[1].Address.String()), check.Equals, true)
	c.Assert(routertest.FakeRouter.HasRoute(app.GetName(), allUnits[2].Address.String()), check.Equals, true)
	c.Assert(routertest.FakeRouter.HasRoute(app.GetName(), allUnits[3].Address.String()), check.Equals, true)
}

func (s *S) TestAddUnitsCopiesTheUnitsSlice(c *check.C) {
	app := NewFakeApp("fiction", "python", 0)
	p := NewFakeProvisioner()
	p.Provision(app)
	defer p.Destroy(app)
	units, err := p.AddUnits(app, 3, "web", nil)
	c.Assert(err, check.IsNil)
	units[0].ID = "something-else"
	c.Assert(units[0].ID, check.Not(check.Equals), p.GetUnits(app)[1].ID)
}

func (s *S) TestAddZeroUnits(c *check.C) {
	p := NewFakeProvisioner()
	units, err := p.AddUnits(nil, 0, "web", nil)
	c.Assert(units, check.IsNil)
	c.Assert(err, check.NotNil)
	c.Assert(err.Error(), check.Equals, "Cannot add 0 units.")
}

func (s *S) TestAddUnitsUnprovisionedApp(c *check.C) {
	app := NewFakeApp("mystic-rhythms", "rush", 0)
	p := NewFakeProvisioner()
	units, err := p.AddUnits(app, 1, "web", nil)
	c.Assert(units, check.IsNil)
	c.Assert(err, check.Equals, errNotProvisioned)
}

func (s *S) TestAddUnitsFailure(c *check.C) {
	p := NewFakeProvisioner()
	p.PrepareFailure("AddUnits", errors.New("Cannot add more units."))
	units, err := p.AddUnits(nil, 10, "web", nil)
	c.Assert(units, check.IsNil)
	c.Assert(err, check.NotNil)
	c.Assert(err.Error(), check.Equals, "Cannot add more units.")
}

func (s *S) TestRemoveUnits(c *check.C) {
	app := NewFakeApp("hemispheres", "rush", 0)
	p := NewFakeProvisioner()
	p.Provision(app)
	_, err := p.AddUnits(app, 5, "web", nil)
	c.Assert(err, check.IsNil)
	oldUnits := p.GetUnits(app)
	buf := bytes.NewBuffer(nil)
	err = p.RemoveUnits(app, 3, "web", buf)
	c.Assert(err, check.IsNil)
	units := p.GetUnits(app)
	c.Assert(units, check.HasLen, 2)
	c.Assert(units[0].ID, check.Equals, "hemispheres-3")
	c.Assert(buf.String(), check.Equals, "removing 3 units")
	c.Assert(units[0].Address.String(), check.Equals, oldUnits[3].Address.String())
	c.Assert(routertest.FakeRouter.HasRoute(app.GetName(), oldUnits[0].Address.String()), check.Equals, false)
	c.Assert(routertest.FakeRouter.HasRoute(app.GetName(), oldUnits[1].Address.String()), check.Equals, false)
	c.Assert(routertest.FakeRouter.HasRoute(app.GetName(), oldUnits[2].Address.String()), check.Equals, false)
	c.Assert(routertest.FakeRouter.HasRoute(app.GetName(), oldUnits[3].Address.String()), check.Equals, true)
}

func (s *S) TestRemoveUnitsDifferentProcesses(c *check.C) {
	app := NewFakeApp("hemispheres", "rush", 0)
	p := NewFakeProvisioner()
	err := p.Provision(app)
	c.Assert(err, check.IsNil)
	_, err = p.AddUnits(app, 5, "p1", nil)
	c.Assert(err, check.IsNil)
	_, err = p.AddUnits(app, 2, "p2", nil)
	c.Assert(err, check.IsNil)
	_, err = p.AddUnits(app, 2, "p3", nil)
	c.Assert(err, check.IsNil)
	err = p.RemoveUnits(app, 2, "p2", nil)
	c.Assert(err, check.IsNil)
	c.Assert(p.GetUnits(app), check.HasLen, 7)
	for i, u := range p.GetUnits(app) {
		if i < 5 {
			c.Assert(u.ProcessName, check.Equals, "p1")
		} else {
			c.Assert(u.ProcessName, check.Equals, "p3")
		}
	}
}

func (s *S) TestRemoveUnitsTooManyUnits(c *check.C) {
	app := NewFakeApp("hemispheres", "rush", 0)
	p := NewFakeProvisioner()
	err := p.Provision(app)
	c.Assert(err, check.IsNil)
	_, err = p.AddUnits(app, 1, "web", nil)
	c.Assert(err, check.IsNil)
	err = p.RemoveUnits(app, 3, "web", nil)
	c.Assert(err, check.NotNil)
	c.Assert(err.Error(), check.Equals, "too many units to remove")
}

func (s *S) TestRemoveUnitsTooManyUnitsOfProcess(c *check.C) {
	app := NewFakeApp("hemispheres", "rush", 0)
	p := NewFakeProvisioner()
	err := p.Provision(app)
	c.Assert(err, check.IsNil)
	_, err = p.AddUnits(app, 1, "web", nil)
	c.Assert(err, check.IsNil)
	_, err = p.AddUnits(app, 4, "worker", nil)
	c.Assert(err, check.IsNil)
	err = p.RemoveUnits(app, 3, "web", nil)
	c.Assert(err, check.NotNil)
	c.Assert(err.Error(), check.Equals, "too many units to remove")
}

func (s *S) TestRemoveUnitsUnprovisionedApp(c *check.C) {
	app := NewFakeApp("tears", "bruce", 0)
	p := NewFakeProvisioner()
	err := p.RemoveUnits(app, 1, "web", nil)
	c.Assert(err, check.Equals, errNotProvisioned)
}

func (s *S) TestRemoveUnitsFailure(c *check.C) {
	p := NewFakeProvisioner()
	p.PrepareFailure("RemoveUnits", errors.New("This program has performed an illegal operation."))
	err := p.RemoveUnits(nil, 0, "web", nil)
	c.Assert(err, check.NotNil)
	c.Assert(err.Error(), check.Equals, "This program has performed an illegal operation.")
}

func (s *S) TestExecuteCommand(c *check.C) {
	var buf bytes.Buffer
	output := []byte("myoutput!")
	app := NewFakeApp("grand-designs", "rush", 2)
	p := NewFakeProvisioner()
	p.PrepareOutput(output)
	p.PrepareOutput(output)
	err := p.ExecuteCommand(&buf, nil, app, "ls", "-l")
	c.Assert(err, check.IsNil)
	cmds := p.GetCmds("ls", app)
	c.Assert(cmds, check.HasLen, 1)
	expected := string(output) + string(output)
	c.Assert(buf.String(), check.Equals, expected)
}

func (s *S) TestExecuteCommandFailureNoOutput(c *check.C) {
	app := NewFakeApp("manhattan-project", "rush", 1)
	p := NewFakeProvisioner()
	p.PrepareFailure("ExecuteCommand", errors.New("Failed to run command."))
	err := p.ExecuteCommand(nil, nil, app, "ls", "-l")
	c.Assert(err, check.NotNil)
	c.Assert(err.Error(), check.Equals, "Failed to run command.")
}

func (s *S) TestExecuteCommandWithOutputAndFailure(c *check.C) {
	var buf bytes.Buffer
	app := NewFakeApp("marathon", "rush", 1)
	p := NewFakeProvisioner()
	p.PrepareFailure("ExecuteCommand", errors.New("Failed to run command."))
	p.PrepareOutput([]byte("myoutput!"))
	err := p.ExecuteCommand(nil, &buf, app, "ls", "-l")
	c.Assert(err, check.NotNil)
	c.Assert(err.Error(), check.Equals, "Failed to run command.")
	c.Assert(buf.String(), check.Equals, "myoutput!")
}

func (s *S) TestExecuteComandTimeout(c *check.C) {
	app := NewFakeApp("territories", "rush", 1)
	p := NewFakeProvisioner()
	err := p.ExecuteCommand(nil, nil, app, "ls -l")
	c.Assert(err, check.NotNil)
	c.Assert(err.Error(), check.Equals, "FakeProvisioner timed out waiting for output.")
}

func (s *S) TestAddr(c *check.C) {
	app := NewFakeApp("quick", "who", 1)
	p := NewFakeProvisioner()
	err := p.Provision(app)
	c.Assert(err, check.IsNil)
	addr, err := p.Addr(app)
	c.Assert(err, check.IsNil)
	c.Assert(addr, check.Equals, "quick.fakerouter.com")
}

func (s *S) TestAddrFailure(c *check.C) {
	p := NewFakeProvisioner()
	p.PrepareFailure("Addr", errors.New("Cannot get addr of this app."))
	addr, err := p.Addr(nil)
	c.Assert(addr, check.Equals, "")
	c.Assert(err, check.NotNil)
	c.Assert(err.Error(), check.Equals, "Cannot get addr of this app.")
}

func (s *S) TestSetCName(c *check.C) {
	app := NewFakeApp("jean", "mj", 0)
	p := NewFakeProvisioner()
	err := p.Provision(app)
	c.Assert(err, check.IsNil)
	err = p.SetCName(app, "cname.com")
	c.Assert(err, check.IsNil)
	c.Assert(p.apps[app.GetName()].cnames, check.DeepEquals, []string{"cname.com"})
	c.Assert(routertest.FakeRouter.HasCName("cname.com"), check.Equals, true)
}

func (s *S) TestSetCNameNotProvisioned(c *check.C) {
	app := NewFakeApp("jean", "mj", 0)
	p := NewFakeProvisioner()
	err := p.SetCName(app, "cname.com")
	c.Assert(err, check.Equals, errNotProvisioned)
}

func (s *S) TestSetCNameFailure(c *check.C) {
	app := NewFakeApp("jean", "mj", 0)
	p := NewFakeProvisioner()
	p.PrepareFailure("SetCName", errors.New("wut"))
	err := p.SetCName(app, "cname.com")
	c.Assert(err, check.NotNil)
	c.Assert(err.Error(), check.Equals, "wut")
}

func (s *S) TestUnsetCName(c *check.C) {
	app := NewFakeApp("jean", "mj", 0)
	p := NewFakeProvisioner()
	err := p.Provision(app)
	c.Assert(err, check.IsNil)
	err = p.SetCName(app, "cname.com")
	c.Assert(err, check.IsNil)
	c.Assert(p.apps[app.GetName()].cnames, check.DeepEquals, []string{"cname.com"})
	c.Assert(routertest.FakeRouter.HasCName("cname.com"), check.Equals, true)
	err = p.UnsetCName(app, "cname.com")
	c.Assert(err, check.IsNil)
	c.Assert(p.HasCName(app, "cname.com"), check.Equals, false)
	c.Assert(routertest.FakeRouter.HasCName("cname.com"), check.Equals, false)
}

func (s *S) TestUnsetCNameNotProvisioned(c *check.C) {
	app := NewFakeApp("jean", "mj", 0)
	p := NewFakeProvisioner()
	err := p.UnsetCName(app, "cname.com")
	c.Assert(err, check.Equals, errNotProvisioned)
}

func (s *S) TestUnsetCNameFailure(c *check.C) {
	app := NewFakeApp("jean", "mj", 0)
	p := NewFakeProvisioner()
	p.PrepareFailure("UnsetCName", errors.New("wut"))
	err := p.UnsetCName(app, "cname.com")
	c.Assert(err, check.NotNil)
	c.Assert(err.Error(), check.Equals, "wut")
}

func (s *S) TestHasCName(c *check.C) {
	app := NewFakeApp("jean", "mj", 0)
	p := NewFakeProvisioner()
	err := p.Provision(app)
	c.Assert(err, check.IsNil)
	err = p.SetCName(app, "cname.com")
	c.Assert(err, check.IsNil)
	c.Assert(p.HasCName(app, "cname.com"), check.Equals, true)
	err = p.UnsetCName(app, "cname.com")
	c.Assert(err, check.IsNil)
	c.Assert(p.HasCName(app, "cname.com"), check.Equals, false)
}

func (s *S) TestExecuteCommandOnce(c *check.C) {
	var buf bytes.Buffer
	output := []byte("myoutput!")
	app := NewFakeApp("grand-designs", "rush", 1)
	p := NewFakeProvisioner()
	p.PrepareOutput(output)
	err := p.ExecuteCommandOnce(&buf, nil, app, "ls", "-l")
	c.Assert(err, check.IsNil)
	cmds := p.GetCmds("ls", app)
	c.Assert(cmds, check.HasLen, 1)
	c.Assert(buf.String(), check.Equals, string(output))
}

func (s *S) TestExtensiblePlatformAdd(c *check.C) {
	p := ExtensibleFakeProvisioner{FakeProvisioner: NewFakeProvisioner()}
	args := map[string]string{"dockerfile": "mydockerfile.txt"}
	err := p.PlatformAdd(provision.PlatformOptions{Name: "python", Args: args})
	c.Assert(err, check.IsNil)
	platform := p.GetPlatform("python")
	c.Assert(platform.Name, check.Equals, "python")
	c.Assert(platform.Version, check.Equals, 1)
	c.Assert(platform.Args, check.DeepEquals, args)
}

func (s *S) TestExtensiblePlatformAddTwice(c *check.C) {
	p := ExtensibleFakeProvisioner{FakeProvisioner: NewFakeProvisioner()}
	args := map[string]string{"dockerfile": "mydockerfile.txt"}
	err := p.PlatformAdd(provision.PlatformOptions{Name: "python", Args: args})
	c.Assert(err, check.IsNil)
	err = p.PlatformAdd(provision.PlatformOptions{Name: "python", Args: nil})
	c.Assert(err, check.NotNil)
	c.Assert(err.Error(), check.Equals, "duplicate platform")
}

func (s *S) TestExtensiblePlatformUpdate(c *check.C) {
	p := ExtensibleFakeProvisioner{FakeProvisioner: NewFakeProvisioner()}
	args := map[string]string{"dockerfile": "mydockerfile.txt"}
	err := p.PlatformAdd(provision.PlatformOptions{Name: "python", Args: args})
	c.Assert(err, check.IsNil)
	args["something"] = "wat"
	err = p.PlatformUpdate(provision.PlatformOptions{Name: "python", Args: args})
	c.Assert(err, check.IsNil)
	platform := p.GetPlatform("python")
	c.Assert(platform.Name, check.Equals, "python")
	c.Assert(platform.Version, check.Equals, 2)
	c.Assert(platform.Args, check.DeepEquals, args)
}

func (s *S) TestExtensiblePlatformUpdateNotFound(c *check.C) {
	p := ExtensibleFakeProvisioner{FakeProvisioner: NewFakeProvisioner()}
	err := p.PlatformUpdate(provision.PlatformOptions{Name: "python", Args: nil})
	c.Assert(err, check.NotNil)
	c.Assert(err.Error(), check.Equals, "platform not found")
}

func (s *S) TestExtensiblePlatformRemove(c *check.C) {
	p := ExtensibleFakeProvisioner{FakeProvisioner: NewFakeProvisioner()}
	args := map[string]string{"dockerfile": "mydockerfile.txt"}
	err := p.PlatformAdd(provision.PlatformOptions{Name: "python", Args: args})
	c.Assert(err, check.IsNil)
	err = p.PlatformRemove("python")
	c.Assert(err, check.IsNil)
	platform := p.GetPlatform("python")
	c.Assert(platform, check.IsNil)
}

func (s *S) TestExtensiblePlatformRemoveNotFound(c *check.C) {
	p := ExtensibleFakeProvisioner{FakeProvisioner: NewFakeProvisioner()}
	err := p.PlatformRemove("python")
	c.Assert(err, check.NotNil)
	c.Assert(err.Error(), check.Equals, "platform not found")
}

func (s *S) TestFakeProvisionerAddUnit(c *check.C) {
	app := NewFakeApp("red-sector", "rush", 1)
	p := NewFakeProvisioner()
	err := p.Provision(app)
	c.Assert(err, check.IsNil)
	p.AddUnit(app, provision.Unit{ID: "red-sector/1"})
	units, err := p.Units(app)
	c.Assert(err, check.IsNil)
	c.Assert(units, check.HasLen, 1)
	c.Assert(p.apps[app.GetName()].unitLen, check.Equals, 1)
}

func (s *S) TestFakeProvisionerUnits(c *check.C) {
	app := NewFakeApp("red-sector", "rush", 1)
	p := NewFakeProvisioner()
	err := p.Provision(app)
	c.Assert(err, check.IsNil)
	p.AddUnit(app, provision.Unit{ID: "red-sector/1"})
	units, err := p.Units(app)
	c.Assert(err, check.IsNil)
	c.Assert(units, check.HasLen, 1)
}

func (s *S) TestFakeProvisionerUnitsAppNotFound(c *check.C) {
	app := NewFakeApp("red-sector", "rush", 1)
	p := NewFakeProvisioner()
	units, err := p.Units(app)
	c.Assert(err, check.IsNil)
	c.Assert(units, check.HasLen, 0)
}

func (s *S) TestFakeProvisionerSetUnitStatus(c *check.C) {
	app := NewFakeApp("red-sector", "rush", 1)
	p := NewFakeProvisioner()
	err := p.Provision(app)
	c.Assert(err, check.IsNil)
	unit := provision.Unit{AppName: "red-sector", ID: "red-sector/1", Status: provision.StatusStarted}
	p.AddUnit(app, unit)
	err = p.SetUnitStatus(unit, provision.StatusError)
	c.Assert(err, check.IsNil)
	units, err := p.Units(app)
	c.Assert(err, check.IsNil)
	unit = units[0]
	c.Assert(unit.Status, check.Equals, provision.StatusError)
}

func (s *S) TestFakeProvisionerSetUnitStatusNoApp(c *check.C) {
	app := NewFakeApp("red-sector", "rush", 1)
	p := NewFakeProvisioner()
	err := p.Provision(app)
	c.Assert(err, check.IsNil)
	unit := provision.Unit{AppName: "red-sector", ID: "red-sector/1", Status: provision.StatusStarted}
	p.AddUnit(app, unit)
	unit = provision.Unit{ID: "red-sector/1"}
	err = p.SetUnitStatus(unit, provision.StatusError)
	c.Assert(err, check.IsNil)
	units, err := p.Units(app)
	c.Assert(err, check.IsNil)
	unit = units[0]
	c.Assert(unit.Status, check.Equals, provision.StatusError)
}

func (s *S) TestFakeProvisionerSetUnitStatusAppNotFound(c *check.C) {
	p := NewFakeProvisioner()
	err := p.SetUnitStatus(provision.Unit{AppName: "something"}, provision.StatusError)
	c.Assert(err, check.Equals, errNotProvisioned)
}

func (s *S) TestFakeProvisionerSetUnitStatusUnitNotFound(c *check.C) {
	app := NewFakeApp("red-sector", "rush", 1)
	p := NewFakeProvisioner()
	err := p.Provision(app)
	c.Assert(err, check.IsNil)
	unit := provision.Unit{AppName: "red-sector", ID: "red-sector/1", Status: provision.StatusStarted}
	err = p.SetUnitStatus(unit, provision.StatusError)
	c.Assert(err, check.NotNil)
	e, ok := err.(*provision.UnitNotFoundError)
	c.Assert(ok, check.Equals, true)
	c.Assert(e.ID, check.Equals, "red-sector/1")
}

func (s *S) TestFakeProvisionerRegisterUnit(c *check.C) {
	app := NewFakeApp("shine-on", "diamond", 1)
	p := NewFakeProvisioner()
	err := p.Provision(app)
	c.Assert(err, check.IsNil)
	unit := provision.Unit{AppName: "shine-on", ID: "unit/1"}
	p.AddUnit(app, unit)
	units, err := p.Units(app)
	c.Assert(err, check.IsNil)
	ip := units[0].Ip
	err = p.RegisterUnit(unit, nil)
	c.Assert(err, check.IsNil)
	units, err = p.Units(app)
	c.Assert(err, check.IsNil)
	c.Assert(units[0].Ip, check.Equals, ip+"-updated")
}

func (s *S) TestFakeProvisionerRegisterUnitNotFound(c *check.C) {
	app := NewFakeApp("shine-on", "diamond", 1)
	p := NewFakeProvisioner()
	err := p.Provision(app)
	c.Assert(err, check.IsNil)
	unit := provision.Unit{AppName: "shine-on", ID: "unit/1"}
	err = p.RegisterUnit(unit, nil)
	c.Assert(err, check.ErrorMatches, "unit not found")
}

func (s *S) TestFakeProvisionerRegisterUnitSavesData(c *check.C) {
	app := NewFakeApp("shine-on", "diamond", 1)
	p := NewFakeProvisioner()
	err := p.Provision(app)
	c.Assert(err, check.IsNil)
	unit := provision.Unit{AppName: "shine-on", ID: "unit/1"}
	p.AddUnit(app, unit)
	units, err := p.Units(app)
	c.Assert(err, check.IsNil)
	ip := units[0].Ip
	data := map[string]interface{}{"my": "data"}
	err = p.RegisterUnit(unit, data)
	c.Assert(err, check.IsNil)
	units, err = p.Units(app)
	c.Assert(err, check.IsNil)
	c.Assert(units[0].Ip, check.Equals, ip+"-updated")
	c.Assert(p.CustomData(app), check.DeepEquals, data)
}

func (s *S) TestFakeProvisionerShellNoSpecification(c *check.C) {
	app := NewFakeApp("shine-on", "diamond", 1)
	p := NewFakeProvisioner()
	err := p.Provision(app)
	c.Assert(err, check.IsNil)
	unit := provision.Unit{AppName: "shine-on", ID: "unit/1"}
	p.AddUnit(app, unit)
	unit = provision.Unit{AppName: "shine-on", ID: "unit/2"}
	p.AddUnit(app, unit)
	unit = provision.Unit{AppName: "shine-on", ID: "unit/3"}
	p.AddUnit(app, unit)
	opts := provision.ShellOptions{App: app}
	err = p.Shell(opts)
	c.Assert(err, check.IsNil)
	c.Assert(p.Shells("unit/1"), check.DeepEquals, []provision.ShellOptions{opts})
	c.Assert(p.Shells("unit/2"), check.HasLen, 0)
	c.Assert(p.Shells("unit/3"), check.HasLen, 0)
}

func (s *S) TestFakeProvisionerShellSpecifying(c *check.C) {
	app := NewFakeApp("shine-on", "diamond", 1)
	p := NewFakeProvisioner()
	err := p.Provision(app)
	c.Assert(err, check.IsNil)
	unit := provision.Unit{AppName: "shine-on", ID: "unit/1"}
	p.AddUnit(app, unit)
	unit = provision.Unit{AppName: "shine-on", ID: "unit/2"}
	p.AddUnit(app, unit)
	unit = provision.Unit{AppName: "shine-on", ID: "unit/3"}
	p.AddUnit(app, unit)
	opts := provision.ShellOptions{App: app, Unit: "unit/3"}
	err = p.Shell(opts)
	c.Assert(err, check.IsNil)
	c.Assert(p.Shells("unit/3"), check.DeepEquals, []provision.ShellOptions{opts})
	c.Assert(p.Shells("unit/1"), check.HasLen, 0)
	c.Assert(p.Shells("unit/2"), check.HasLen, 0)
}

func (s *S) TestFakeProvisionerShellUnitNotFound(c *check.C) {
	app := NewFakeApp("shine-on", "diamond", 1)
	p := NewFakeProvisioner()
	err := p.Provision(app)
	c.Assert(err, check.IsNil)
	unit := provision.Unit{AppName: "shine-on", ID: "unit/1"}
	p.AddUnit(app, unit)
	opts := provision.ShellOptions{App: app, Unit: "unit/12"}
	err = p.Shell(opts)
	c.Assert(err.Error(), check.Equals, "unit not found")
}

func (s *S) TestFakeProvisionerShellNoUnits(c *check.C) {
	app := NewFakeApp("shine-on", "diamond", 1)
	p := NewFakeProvisioner()
	err := p.Provision(app)
	c.Assert(err, check.IsNil)
	opts := provision.ShellOptions{App: app}
	err = p.Shell(opts)
	c.Assert(err.Error(), check.Equals, "app has no units")
}

func (s *S) TestFakeProvisionerMetricEnvs(c *check.C) {
	app := NewFakeApp("shine-on", "diamond", 1)
	p := NewFakeProvisioner()
	envs := p.MetricEnvs(app)
	expected := map[string]string{"METRICS_BACKEND": "fake"}
	c.Assert(envs, check.DeepEquals, expected)
}

func (s *S) TestFakeProvisionerAddNode(c *check.C) {
	p := NewFakeProvisioner()
	p.AddNode("mynode", "mypool")
	c.Assert(p.nodes, check.DeepEquals, map[string]fakeNode{"mynode": {address: "mynode", pool: "mypool"}})
}

type NodeList []provision.Node

func (l NodeList) Len() int           { return len(l) }
func (l NodeList) Swap(i, j int)      { l[i], l[j] = l[j], l[i] }
func (l NodeList) Less(i, j int) bool { return l[i].Address() < l[j].Address() }

func (s *S) TestFakeProvisionerListNodes(c *check.C) {
	p := NewFakeProvisioner()
	p.AddNode("mynode1", "mypool")
	p.AddNode("mynode2", "mypool")
	nodes, err := p.ListNodes(nil)
	c.Assert(err, check.IsNil)
	sort.Sort(NodeList(nodes))
	c.Assert(nodes, check.DeepEquals, []provision.Node{
		&fakeNode{address: "mynode1", pool: "mypool"},
		&fakeNode{address: "mynode2", pool: "mypool"},
	})
	nodes, err = p.ListNodes([]string{"mynode2"})
	c.Assert(err, check.IsNil)
	c.Assert(nodes, check.DeepEquals, []provision.Node{
		&fakeNode{address: "mynode2", pool: "mypool"},
	})
}

func (s *S) TestFakeProvisionerFilterAppsByUnitStatus(c *check.C) {
	app1 := NewFakeApp("fairy-tale", "shaman", 1)
	app2 := NewFakeApp("unfairy-tale", "shaman", 1)
	p := NewFakeProvisioner()
	err := p.Provision(app1)
	c.Assert(err, check.IsNil)
	err = p.Provision(app2)
	c.Assert(err, check.IsNil)
	unit := provision.Unit{AppName: "fairy-tale", ID: "unit/1", Status: provision.StatusStarting}
	p.AddUnit(app1, unit)
	unit = provision.Unit{AppName: "unfairy-tale", ID: "unit/2", Status: provision.StatusStarting}
	p.AddUnit(app2, unit)
	err = p.SetUnitStatus(unit, provision.StatusError)
	c.Assert(err, check.IsNil)
	apps, err := p.FilterAppsByUnitStatus([]provision.App{app1, app2}, []string{"starting"})
	c.Assert(apps, check.DeepEquals, []provision.App{app1})
	c.Assert(err, check.IsNil)
}

func (s *S) TestGetAppFromUnitID(c *check.C) {
	list := []provision.Unit{
		{ID: "chain-lighting-0", AppName: "chain-lighting", ProcessName: "web", Type: "django", Ip: "10.10.10.10", Status: provision.StatusStarted},
	}
	app := NewFakeApp("chain-lighting", "rush", 1)
	p := NewFakeProvisioner()
	p.apps = map[string]provisionedApp{
		app.GetName(): {app: app, units: list},
	}
	a, err := p.GetAppFromUnitID("chain-lighting-0")
	c.Assert(err, check.IsNil)
	c.Assert(app, check.DeepEquals, a)
}

func (s *S) TestGetAppFromUnitIDNotFound(c *check.C) {
	p := NewFakeProvisioner()
	_, err := p.GetAppFromUnitID("chain-lighting-0")
	c.Assert(err, check.NotNil)
}
