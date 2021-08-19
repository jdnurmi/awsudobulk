package main

import (
	"context"
	"flag"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/golang/glog"
	"gopkg.in/ini.v1"
	"time"
)

var (
	refresh_padding time.Duration
)

// Quick & Dirty helper func to return an aws.CredentialsProvider from a set of
// known aws credentials
func GetCredsFunc(in aws.Credentials) func(context.Context) (aws.Credentials, error) {
	return func(context.Context) (aws.Credentials, error) {
		return in, nil
	}
}

func main() {
	home, _ := os.UserHomeDir()
	var (
		glob         string
		command      string
		command_args []string
		load_cache	bool
		save_cache	bool
		cache_file   = filepath.Join(home, ".aws", "cli", "cache", "awsudo")
		authCache = NewAwsAuthCache()
	)
	flag.BoolVar(&load_cache, "load-cache", true, "Load cached authentication data")
	flag.BoolVar(&save_cache, "save-cache", true, "Save cached authentication data (only on success)")
	flag.DurationVar(&refresh_padding, "refresh-padding", time.Minute*5, "Refresh credentials with less than this time left")
	flag.StringVar(&glob, "u", "", "Credential to match (globbing supported)")
	flag.StringVar(&cache_file, "cache", cache_file, "File to cache credentials in")
	flag.Parse()

	if load_cache {
		authCache = LoadAwsAuthCache(cache_file)
	}

	if flag.NArg() < 1 {
		glog.Errorf("Usage: %s COMMAND [command arguments]", os.Args[0])
		os.Exit(1)
	}

	command, command_args = flag.Arg(0), flag.Args()[1:]

	cf, err := LoadDefaultCredentials()
	if err != nil {
		glog.Fatalf("Couldn't load credentials: %v", err)
	}
	type CredSet struct {
		Name   string
		Region string
		Creds  aws.Credentials
	}
	credSets := []CredSet{}

	err = cf.EachProfileSection(func(name string, secs ...*ini.Section) (err error) {
		var (
			creds aws.Credentials

			_region string
			// SSO Access
			sso_start_url string
			sso_region    string
			sso_account   string
			sso_role_name string
		)
		if ok, _ := filepath.Match(glob, name); !ok || name == "DEFAULT" {
			return
		}

		sso_account, _ = GetValueString("sso_account_id", secs...)
		sso_start_url, _ = GetValueString("sso_start_url", secs...)
		sso_region, _ = GetValueString("sso_region", secs...)
		sso_role_name, _ = GetValueString("sso_role_name", secs...)
		_region, _ = GetValueString("region", secs...)

		creds, err = SSOPathCredentials(context.TODO(), authCache, sso_start_url, sso_region, sso_account, sso_role_name)
		if err != nil {
			glog.Errorf("[%s] Error loading sso credentials: %v", name, err)
			return err
		}
		credSets = append(credSets, CredSet{name, _region, creds})
		return
	})
	if err != nil {
		os.Exit(1)
	}

	// We look through each section, and if name matches glob,
	// we initialize appropriate credentials and then execute
	// the given command.
	err = cf.EachCredentialSection(func(name string, secs ...*ini.Section) (err error) {
		var (
			creds aws.Credentials
			// Standard access
			_region     string
			_access     string
			_secret     string
			_role_arn   string
			_mfa_serial string
		)
		if ok, _ := filepath.Match(glob, name); !ok || name == "DEFAULT" {
			return nil
		}
		_access, _ = GetValueString("aws_access_key_id", secs...)
		_secret, _ = GetValueString("aws_secret_access_key", secs...)
		_region, _ = GetValueString("region", secs...)
		_role_arn, _ = GetValueString("role_arn", secs...)
		_mfa_serial, _ = GetValueString("mfa_serial", secs...)

		creds, err = ConfigPathCredentials(authCache, _access, _secret, _region, _role_arn, _mfa_serial)
		if err != nil {
			glog.Errorf("[%s] Error loading credentials: %v", name, err)
			return err
		}
		credSets = append(credSets, CredSet{name, _region, creds})
		return
	})
	if err != nil {
		os.Exit(1)
	}

	for _, credSet := range credSets {
		glog.V(1).Infof("[%s] Running %q %v", credSet.Name, command, command_args)
		cmd := exec.Command(command, command_args...)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		tgt_env := map[string]string{
			"AWS_ACCESS_KEY_ID":     credSet.Creds.AccessKeyID,
			"AWS_SECRET_ACCESS_KEY": credSet.Creds.SecretAccessKey,
			"AWS_SESSION_TOKEN":     credSet.Creds.SessionToken,
		}
		// only override the current region if the config defines one
		if credSet.Region != "" {
			tgt_env["AWS_DEFAULT_REGION"] = credSet.Region
		}
		cmd.Env = CloneFreshEnv(tgt_env)

		err = cmd.Run()
		if err != nil {
			break
		}
	}
	if err != nil {
		glog.Fatalf("Error in execution: %v", err)
	} else if save_cache {
		err = authCache.SaveCache(cache_file)
		if err != nil {
			glog.Fatalf("Error saving credentials cache: %v", err)
		}
	}
}
