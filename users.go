package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/tucats/gopackages/app-cli/cli"
	"github.com/tucats/gopackages/app-cli/persistence"
	"github.com/tucats/gopackages/app-cli/ui"
	"github.com/tucats/gopackages/symbols"
	"github.com/tucats/gopackages/util"
)

var userDatabase map[string]string

// loadUserDatabase uses command line options to locate and load the authorized users
// database, or initialize it to a helpful default.
func loadUserDatabase(c *cli.Context) error {

	defaultUser := "admin"
	defaultPassword := "password"
	if up := persistence.Get("default-credential"); up != "" {
		if pos := strings.Index(up, ":"); pos >= 0 {
			defaultUser = up[:pos]
			defaultPassword = up[pos+1:]
		} else {
			defaultUser = up
			defaultPassword = ""
		}
	}

	// Is there a user database to load?
	userFile, _ := c.GetString("users")
	if userFile == "" {
		userFile = persistence.Get("logon-userdata")
	}
	if userFile != "" {
		b, err := ioutil.ReadFile(userFile)
		if err == nil {
			err = json.Unmarshal(b, &userDatabase)
		}
		if err != nil {
			return err
		}
		ui.Debug(ui.ServerLogger, "Using stored credentials with %d items", len(userDatabase))
	} else {
		userDatabase = map[string]string{
			defaultUser: defaultPassword,
		}
		ui.Debug(ui.ServerLogger, "Using default credentials %s:%s", defaultUser, defaultPassword)
	}

	if su := persistence.Get("logon-superuser"); su != "" {
		userDatabase[su] = ""
		ui.Debug(ui.ServerLogger, "Adding superuser to user database")
	}
	return nil
}

// Authenticated implmeents the Authenticated(user,pass) function. This accepts a username
// and password string, and determines if they are authenticated using the
// users database.
func Authenticated(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {

	var user, pass string

	// If there are no arguments, then we look for the _user and _password
	// variables and use those. Otherwise, fetch them as the two parameters.
	if len(args) == 0 {
		if ux, ok := s.Get("_user"); ok {
			user = util.GetString(ux)
		}
		if px, ok := s.Get("_password"); ok {
			pass = util.GetString(px)
		}
	} else {
		if len(args) != 2 {
			return false, fmt.Errorf("incorrect number of arguments")
		}
		user = util.GetString(args[0])
		pass = util.GetString(args[1])
	}

	// If no user database, then we're done.
	if userDatabase == nil {
		return false, nil
	}

	// If the user exists and the password matches then valid.
	if p, ok := userDatabase[user]; ok && p == pass {
		return true, nil
	}
	return false, nil
}
