package main

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/golang/glog"
)

func ConfigPathCredentials(authCache *AwsAuthCacher,
	_access, _secret, _region, _role_arn, _mfa_serial string) (creds aws.Credentials, err error) {

	// Seed the creds with the narrowest-scoped credentials
	creds = aws.Credentials{
		AccessKeyID:     _access,
		SecretAccessKey: _secret,
	}

	// Setup a configuration to use those creds and the narrowest-scoped region
	config := aws.Config{
		Region:      _region,
		Credentials: aws.CredentialsProviderFunc(GetCredsFunc(creds)),
	}

	// If we know of the MFA, let's get it into our creds.
	if _mfa_serial != "" {
		glog.V(2).Infof("Fetching MFA credentials for %q", _mfa_serial)
		creds, err = authCache.GetMfaCredentials(config, _mfa_serial)
		// Update the config to use the credentials we just got from using the MFA
		if err == nil {
			config.Credentials = aws.CredentialsProviderFunc(GetCredsFunc(creds))
		}
	}

	// If we need to assume a role, we need to switch into it using our GST creds
	if err == nil && _role_arn != "" {
		glog.V(2).Infof("Fetching Role credentials for %q", _role_arn)
		creds, err = authCache.GetRoleCredentials(config, _role_arn)
		// If there were further steps, we'd update our config as well, but this is the
		// end, we just go from here.
	}

	return
}

func SSOPathCredentials(ctx context.Context, authCache *AwsAuthCacher, start, region, account, role string) (creds aws.Credentials, err error) {
	// Start in the SSO region
	config := aws.Config{
		Region: region,
	}
	creds, err = authCache.GetSSOCredentials(ctx, config, start, account, role, func(auth string) (err error) {
		fmt.Printf("Visit %q to authorize this application\n", auth)
		time.Sleep(time.Second * 5)
		return nil
	})
	return
}
