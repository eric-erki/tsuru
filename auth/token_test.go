// Copyright 2013 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package auth

import (
	"crypto"
	"encoding/json"
	"fmt"
	"labix.org/v2/mgo/bson"
	"launchpad.net/gocheck"
	"sync"
	"time"
)

func (s *S) TestTokenCannotRepeat(c *gocheck.C) {
	input := "user-token"
	tokens := make([]string, 10)
	var wg sync.WaitGroup
	for i := range tokens {
		wg.Add(1)
		go func(i int) {
			tokens[i] = token(input, crypto.MD5)
			wg.Done()
		}(i)
	}
	wg.Wait()
	reference := tokens[0]
	for _, t := range tokens[1:] {
		c.Check(t, gocheck.Not(gocheck.Equals), reference)
	}
}

func (s *S) TestNewTokenReturnsErroWhenUserReferenceDoesNotContainsEmail(c *gocheck.C) {
	u := User{}
	t, err := newUserToken(&u)
	c.Assert(t, gocheck.IsNil)
	c.Assert(err, gocheck.NotNil)
	c.Assert(err, gocheck.ErrorMatches, "^Impossible to generate tokens for users without email$")
}

func (s *S) TestNewTokenReturnsErrorWhenUserIsNil(c *gocheck.C) {
	t, err := newUserToken(nil)
	c.Assert(t, gocheck.IsNil)
	c.Assert(err, gocheck.NotNil)
	c.Assert(err, gocheck.ErrorMatches, "^User is nil$")
}

func (s *S) TestGetToken(c *gocheck.C) {
	t, err := GetToken(s.token.Token)
	c.Assert(err, gocheck.IsNil)
	c.Assert(t.Token, gocheck.Equals, s.token.Token)
}

func (s *S) TestGetTokenEmptyToken(c *gocheck.C) {
	u, err := GetToken("")
	c.Assert(u, gocheck.IsNil)
	c.Assert(err, gocheck.NotNil)
	c.Assert(err.Error(), gocheck.Equals, "Token not found")
}

func (s *S) TestGetTokenNotFound(c *gocheck.C) {
	t, err := GetToken("invalid")
	c.Assert(t, gocheck.IsNil)
	c.Assert(err, gocheck.NotNil)
	c.Assert(err, gocheck.ErrorMatches, "Token not found")
}

func (s *S) TestGetExpiredToken(c *gocheck.C) {
	t, err := CreateApplicationToken("tsuru-healer")
	c.Assert(err, gocheck.IsNil)
	defer s.conn.Tokens().Remove(bson.M{"token": t.Token})
	t.ValidUntil = time.Now().Add(-24 * time.Hour)
	s.conn.Tokens().Update(bson.M{"token": t.Token}, t)
	t2, err := GetToken(t.Token)
	c.Assert(t2, gocheck.IsNil)
	c.Assert(err, gocheck.NotNil)
	c.Assert(err.Error(), gocheck.Equals, "Token has expired")
}

func (s *S) TestCreateApplicationToken(c *gocheck.C) {
	t, err := CreateApplicationToken("tsuru-healer")
	c.Assert(err, gocheck.IsNil)
	c.Assert(t, gocheck.NotNil)
	defer s.conn.Tokens().Remove(bson.M{"token": t.Token})
	n, err := s.conn.Tokens().Find(t).Count()
	c.Assert(err, gocheck.IsNil)
	c.Assert(n, gocheck.Equals, 1)
	c.Assert(t.AppName, gocheck.Equals, "tsuru-healer")
}

func (s *S) TestTokenMarshalJSON(c *gocheck.C) {
	valid := time.Now()
	t := Token{
		Token:      "12saii",
		ValidUntil: valid,
		UserEmail:  "something@something.com",
		AppName:    "myapp",
	}
	b, err := json.Marshal(&t)
	c.Assert(err, gocheck.IsNil)
	want := fmt.Sprintf(`{"token":"12saii","valid-until":"%s","email":"something@something.com","app":"myapp"}`,
		valid.Format(time.RFC3339Nano))
	c.Assert(string(b), gocheck.Equals, want)
}

func (s *S) TestTokenGetUser(c *gocheck.C) {
	u, err := s.token.User()
	c.Assert(err, gocheck.IsNil)
	c.Assert(u.Email, gocheck.Equals, s.user.Email)
}

func (s *S) TestTokenGetUserUnknownEmail(c *gocheck.C) {
	t := Token{UserEmail: "something@something.com"}
	u, err := t.User()
	c.Assert(u, gocheck.IsNil)
	c.Assert(err, gocheck.NotNil)
}

func (s *S) TestDeleteToken(c *gocheck.C) {
	t, err := CreateApplicationToken("tsuru-healer")
	c.Assert(err, gocheck.IsNil)
	err = DeleteToken(t.Token)
	c.Assert(err, gocheck.IsNil)
	_, err = GetToken(t.Token)
	c.Assert(err, gocheck.NotNil)
	c.Assert(err.Error(), gocheck.Equals, "Token not found")
}

func (s *S) TestCreatePasswordToken(c *gocheck.C) {
	u := User{Email: "pure@alanis.com"}
	t, err := createPasswordToken(&u)
	c.Assert(err, gocheck.IsNil)
	c.Assert(t.UserEmail, gocheck.Equals, u.Email)
	c.Assert(t.Used, gocheck.Equals, false)
	var dbToken PasswordToken
	err = s.conn.PasswordTokens().Find(bson.M{"_id": t.Token}).One(&dbToken)
	c.Assert(err, gocheck.IsNil)
	c.Assert(dbToken.Token, gocheck.Equals, t.Token)
	c.Assert(dbToken.UserEmail, gocheck.Equals, t.UserEmail)
	c.Assert(dbToken.Used, gocheck.Equals, t.Used)
}

func (s *S) TestCreatePasswordTokenErrors(c *gocheck.C) {
	var tests = []struct {
		input *User
		want  string
	}{
		{nil, "User is nil"},
		{&User{}, "User email is empty"},
	}
	for _, t := range tests {
		token, err := createPasswordToken(t.input)
		c.Check(token, gocheck.IsNil)
		c.Check(err, gocheck.NotNil)
		c.Check(err.Error(), gocheck.Equals, t.want)
	}
}
