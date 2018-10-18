// Copyright 2016 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package admincmd_test

import (
	"path/filepath"
	"time"

	"github.com/CanonicalLtd/candid/store"
	"golang.org/x/net/context"
	gc "gopkg.in/check.v1"
	"gopkg.in/macaroon-bakery.v2/bakery"
)

type showSuite struct {
	commandSuite
}

var _ = gc.Suite(&showSuite{})

func (s *showSuite) TestShowUserWithAgentEnv(c *gc.C) {
	// This test acts as a proxy agent-env functionality in all the
	// other command that use NewClient.
	s.PatchEnvironment("BAKERY_AGENT_FILE", filepath.Join(s.Dir, "admin.agent"))
	ctx := context.Background()
	s.server.AddIdentity(ctx, &store.Identity{
		ProviderID: store.MakeProviderIdentity("test", "bob"),
		Username:   "bob",
	})
	stdout := s.CheckSuccess(c, "show", "-u", "bob")
	c.Assert(stdout, gc.Equals, `
username: bob
external-id: test:bob
name: ""
email: ""
groups: []
ssh-keys: []
last-login: never
last-discharge: never
`[1:])
}

func (s *showSuite) TestShowUser(c *gc.C) {
	ctx := context.Background()
	s.server.AddIdentity(ctx, &store.Identity{
		ProviderID:    store.MakeProviderIdentity("test", "bob"),
		Username:      "bob",
		Name:          "Bob Robertson",
		Email:         "bob@example.com",
		Groups:        []string{"g1", "g2"},
		LastLogin:     time.Date(2016, 12, 25, 0, 0, 0, 0, time.UTC),
		LastDischarge: time.Date(2016, 12, 25, 0, 0, 0, 0, time.UTC),
		ExtraInfo: map[string][]string{
			"sshkeys": {"key1", "key2"},
		},
	})
	stdout := s.CheckSuccess(c, "show", "-a", "admin.agent", "-u", "bob")
	c.Assert(stdout, gc.Equals, `
username: bob
external-id: test:bob
name: Bob Robertson
email: bob@example.com
groups:
- g1
- g2
ssh-keys:
- key1
- key2
last-login: "2016-12-25T00:00:00Z"
last-discharge: "2016-12-25T00:00:00Z"
`[1:])
}

func (s *showSuite) TestShowEmail(c *gc.C) {
	ctx := context.Background()
	s.server.AddIdentity(ctx, &store.Identity{
		ProviderID:    store.MakeProviderIdentity("test", "bob"),
		Username:      "bob",
		Name:          "Bob Robertson",
		Email:         "bob@example.com",
		Groups:        []string{"g1", "g2"},
		LastLogin:     time.Date(2016, 12, 25, 0, 0, 0, 0, time.UTC),
		LastDischarge: time.Date(2016, 12, 25, 0, 0, 0, 0, time.UTC),
		ExtraInfo: map[string][]string{
			"sshkeys": {"key1", "key2"},
		},
	})
	stdout := s.CheckSuccess(c, "show", "-a", "admin.agent", "-e", "bob@example.com")
	c.Assert(stdout, gc.Equals, `
username: bob
external-id: test:bob
name: Bob Robertson
email: bob@example.com
groups:
- g1
- g2
ssh-keys:
- key1
- key2
last-login: "2016-12-25T00:00:00Z"
last-discharge: "2016-12-25T00:00:00Z"
`[1:])
}

func (s *showSuite) TestShowEmailNotFound(c *gc.C) {
	s.CheckError(
		c,
		1,
		`no user found for email "bob@example.com"`,
		"show", "-a", "admin.agent", "-e", "bob@example.com",
	)
}

func (s *showSuite) TestShowNoParameters(c *gc.C) {
	s.CheckError(
		c,
		2,
		`no user specified, please specify either username or email`,
		"show",
	)
}

func (s *showSuite) TestShowAgentUser(c *gc.C) {
	ctx := context.Background()
	var pk bakery.PublicKey
	identities := []store.Identity{{
		ProviderID: store.MakeProviderIdentity("static", "alice"),
		Username:   "alice",
		Groups:     []string{"g1", "g2"},
	}, {
		ProviderID:    store.MakeProviderIdentity("idm", "a-1234"),
		Username:      "a-1234@candid",
		PublicKeys:    []bakery.PublicKey{pk},
		Groups:        []string{"g1", "g2"},
		Owner:         store.MakeProviderIdentity("static", "alice"),
		LastLogin:     time.Date(2016, 12, 25, 0, 0, 0, 0, time.UTC),
		LastDischarge: time.Date(2016, 12, 25, 0, 0, 0, 0, time.UTC),
	}}
	for _, id := range identities {
		s.server.AddIdentity(ctx, &id)
	}
	stdout := s.CheckSuccess(c, "show", "-a", "admin.agent", "-u", "a-1234@candid")
	c.Assert(stdout, gc.Equals, `
username: a-1234@candid
owner: alice
public-keys:
- AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=
groups:
- g1
- g2
ssh-keys: []
last-login: "2016-12-25T00:00:00Z"
last-discharge: "2016-12-25T00:00:00Z"
`[1:])
}

func (s *showSuite) TestShowZeroValues(c *gc.C) {
	ctx := context.Background()
	s.server.AddIdentity(ctx, &store.Identity{
		ProviderID: store.MakeProviderIdentity("test", "bob"),
		Username:   "bob",
	})
	stdout := s.CheckSuccess(c, "show", "-a", "admin.agent", "-u", "bob")
	c.Assert(stdout, gc.Equals, `
username: bob
external-id: test:bob
name: ""
email: ""
groups: []
ssh-keys: []
last-login: never
last-discharge: never
`[1:])
}

func (s *showSuite) TestShowUserError(c *gc.C) {
	s.CheckError(
		c,
		1,
		`Get http://.*/v1/u/bob: user bob not found`,
		"show", "-a", "admin.agent", "-u", "bob",
	)
}

func (s *showSuite) TestShowUserJSON(c *gc.C) {
	ctx := context.Background()
	s.server.AddIdentity(ctx, &store.Identity{
		ProviderID:    store.MakeProviderIdentity("test", "bob"),
		Username:      "bob",
		Name:          "Bob Robertson",
		Email:         "bob@example.com",
		Groups:        []string{"g1", "g2"},
		LastLogin:     time.Date(2016, 12, 25, 0, 0, 0, 0, time.UTC),
		LastDischarge: time.Date(2016, 12, 25, 0, 0, 0, 0, time.UTC),
		ExtraInfo: map[string][]string{
			"sshkeys": {"key1", "key2"},
		},
	})
	stdout := s.CheckSuccess(c, "show", "-a", "admin.agent", "-u", "bob", "--format", "json")
	c.Assert(stdout, gc.Equals, `
{"username":"bob","external-id":"test:bob","name":"Bob Robertson","email":"bob@example.com","groups":["g1","g2"],"ssh-keys":["key1","key2"],"last-login":"2016-12-25T00:00:00Z","last-discharge":"2016-12-25T00:00:00Z"}
`[1:])
}
