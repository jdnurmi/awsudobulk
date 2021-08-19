package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sso"
	"github.com/aws/aws-sdk-go-v2/service/ssooidc"
	"github.com/aws/aws-sdk-go-v2/service/ssooidc/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/golang/glog"
)

type AwsAuthCacher struct {
	// Map of MFA's or RoleID's to credentials
	Credentials map[string]aws.Credentials

	// Map of regions to client registrations
	SSORegistrations map[string]*ssooidc.RegisterClientOutput
	// Map of start-url's to client authorizations
	SSOAuthorizations map[string]*ssooidc.StartDeviceAuthorizationOutput
	// Map of start-url's to SSO tokens
	SSOTokens map[string]*ssooidc.CreateTokenOutput
}

// Loads cached credentials from 'fn'
func LoadAwsAuthCache(fn string) (cacher *AwsAuthCacher) {
	cacher = &AwsAuthCacher{
		Credentials:       map[string]aws.Credentials{},
		SSORegistrations:  map[string]*ssooidc.RegisterClientOutput{},
		SSOAuthorizations: map[string]*ssooidc.StartDeviceAuthorizationOutput{},
		SSOTokens:         map[string]*ssooidc.CreateTokenOutput{},
	}
	fp, err := os.Open(fn)
	if err == nil {
		defer fp.Close()
		err = json.NewDecoder(fp).Decode(cacher)
	}
	return
}

func (h AwsAuthCacher) SaveCache(fn string) (err error) {
	fp, err := os.OpenFile(fn, os.O_CREATE|os.O_RDWR, 0600)
	if err == nil {
		defer fp.Close()
		err = json.NewEncoder(fp).Encode(h)
	}
	return
}

func (h AwsAuthCacher) GetMfaCredentials(conf aws.Config, serial string) (creds aws.Credentials, err error) {
	var ok bool
	if creds, ok = h.Credentials[serial]; ok && creds.Expires.Add(refresh_padding).After(time.Now()) {
		glog.V(3).Infof("Returning cached MFA credentials for %q", serial)
		return
	}
	glog.V(3).Infof("No valid cached MFA credentials for %q, refreshing", serial)
	fmt.Fprintf(os.Stderr, "Need MFA token for %q: ", serial)
	stsc := sts.NewFromConfig(conf)
	reader := bufio.NewReader(os.Stdin)
	token, _ := reader.ReadString('\n')
	token = strings.TrimSpace(token)

	sts_req := &sts.GetSessionTokenInput{
		SerialNumber: &serial,
		TokenCode:    &token,
	}
	sts_out, err := stsc.GetSessionToken(context.Background(), sts_req)
	if err != nil {
		return
	}
	h.Credentials[serial] = aws.Credentials{
		AccessKeyID:     *sts_out.Credentials.AccessKeyId,
		SecretAccessKey: *sts_out.Credentials.SecretAccessKey,
		SessionToken:    *sts_out.Credentials.SessionToken,
		CanExpire:       true,
		Expires:         *sts_out.Credentials.Expiration,
		Source:          "MFA:" + serial,
	}
	return h.Credentials[serial], err
}

func (h AwsAuthCacher) GetRoleCredentials(conf aws.Config, role string) (creds aws.Credentials, err error) {
	var ok bool
	if creds, ok = h.Credentials[role]; ok && creds.Expires.Add(refresh_padding).After(time.Now()) {
		glog.V(3).Infof("Returning cached MFA credentials for %q", role)
		return
	}
	glog.V(3).Infof("No valid cached MFA credentials for %q, refreshing", role)
	stsc := sts.NewFromConfig(conf)
	assume_req := &sts.AssumeRoleInput{
		RoleArn:         &role,
		RoleSessionName: aws.String("awsudo"),
		// external-id
		// durationseconds
		// policy
		// policy-arns
		// serial-number
		// source-identity
		// tags
		// token-code
		// transitive-tags
	}
	assume_resp, err := stsc.AssumeRole(context.Background(), assume_req)
	if err != nil {
		return
	}
	h.Credentials[role] = aws.Credentials{
		AccessKeyID:     *assume_resp.Credentials.AccessKeyId,
		SecretAccessKey: *assume_resp.Credentials.SecretAccessKey,
		SessionToken:    *assume_resp.Credentials.SessionToken,
		CanExpire:       true,
		Expires:         *assume_resp.Credentials.Expiration,
		Source:          "SESS:" + role,
	}
	return h.Credentials[role], err
}

func (h AwsAuthCacher) getSSORegistration(ctx context.Context, conf aws.Config) (reg *ssooidc.RegisterClientOutput, err error) {
	var ok bool
	if reg, ok = h.SSORegistrations[conf.Region]; !ok {
		reg, err = ssooidc.NewFromConfig(conf).RegisterClient(ctx, &ssooidc.RegisterClientInput{
			ClientName: aws.String("awsudobulk"),
			ClientType: aws.String("public"),
		})
		if err == nil {
			h.SSORegistrations[conf.Region] = reg
		}
	}
	return
}

func (h AwsAuthCacher) getSSOAuthorization(ctx context.Context, conf aws.Config, start string) (auth *ssooidc.StartDeviceAuthorizationOutput, err error) {
	var ok bool
	if auth, ok = h.SSOAuthorizations[start]; !ok {
		var reg *ssooidc.RegisterClientOutput
		reg, err = h.getSSORegistration(ctx, conf)
		if err != nil {
			return
		}
		auth, err = ssooidc.NewFromConfig(conf).StartDeviceAuthorization(ctx, &ssooidc.StartDeviceAuthorizationInput{
			ClientId:     reg.ClientId,
			ClientSecret: reg.ClientSecret,
			StartUrl:     &start,
		})
		if err == nil {
			h.SSOAuthorizations[start] = auth
		}
	}
	return
}

func (h AwsAuthCacher) getSSOToken(ctx context.Context, conf aws.Config, start string) (token *ssooidc.CreateTokenOutput, err error) {
	var ok bool
	if token, ok = h.SSOTokens[start]; !ok {
		var auth *ssooidc.StartDeviceAuthorizationOutput
		var reg *ssooidc.RegisterClientOutput
		reg, err = h.getSSORegistration(ctx, conf)
		if err != nil {
			return
		}
		auth, err = h.getSSOAuthorization(ctx, conf, start)
		if err != nil {
			return
		}
		token, err = ssooidc.NewFromConfig(conf).CreateToken(ctx, &ssooidc.CreateTokenInput{
			ClientId:     reg.ClientId,
			ClientSecret: reg.ClientSecret,
			DeviceCode:   auth.DeviceCode,
			GrantType:    aws.String("urn:ietf:params:oauth:grant-type:device_code"),
			// code, redirecturl
		})
		// Handle Token Error
		if err == nil {
			h.SSOTokens[start] = token
		}
	}
	return
}

// creds, err = authCache.GetSSOCredentials(ctx, config, start, account, role, func(auth string)(err error){
func (h AwsAuthCacher) GetSSOCredentials(ctx context.Context, conf aws.Config, start, account, role string, cb func(string) error) (creds aws.Credentials, err error) {
	var token *ssooidc.CreateTokenOutput
	var ok bool
	cache_key := fmt.Sprintf("SSO:%s:%s:%s", start, account, role)
	if creds, ok = h.Credentials[cache_key]; ok && creds.Expires.Add(refresh_padding).After(time.Now()) {
		return
	}
	for token == nil && err == nil {
		token, err = h.getSSOToken(ctx, conf, start)
		if err == nil {
			break
		}
		var waiting *types.AuthorizationPendingException
		if errors.As(err, &waiting) {
			var auth *ssooidc.StartDeviceAuthorizationOutput
			auth, err = h.getSSOAuthorization(ctx, conf, start)
			if err == nil {
				err = cb(*auth.VerificationUriComplete)
			}
		} else {
			break
		}
	}
	if err != nil {
		return
	}
	rco, err := sso.NewFromConfig(conf).GetRoleCredentials(ctx, &sso.GetRoleCredentialsInput{
		AccessToken: token.AccessToken,
		AccountId:   &account,
		RoleName:    &role,
	})
	if err == nil {
		creds = aws.Credentials{
			AccessKeyID:     *rco.RoleCredentials.AccessKeyId,
			SecretAccessKey: *rco.RoleCredentials.SecretAccessKey,
			SessionToken:    *rco.RoleCredentials.SessionToken,
			Expires:         time.Unix(rco.RoleCredentials.Expiration/1000, 0),
			CanExpire:       true,
		}
		h.Credentials[cache_key] = creds
	}
	return
}
