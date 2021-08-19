# AWSUDOBULK

Inspired by awsudo, and one too many AWS accounts.

## What is it?

awsudobulk is a command-line handler to enable performing work across a number of AWS accounts in one command line run.

## How do I configure it

Assuming you've already configured awscli, you're probably pretty close to done.

For AWS-SSO accounts, ensure that you have appropriate profile block(s) in ~/.aws/config maybe like:

    [profile company-root]
    sso_account_id = 2XXXXXXXXXXX
    sso_start_url = https://my-sso-base-url.awsapps.com/start
    sso_region = eu-central-1
    sso_role_name = AdministratorAccess
    region = eu-west-1

    [profile company-logs]
    sso_account_id = 3XXXXXXXXXXX
    sso_start_url = https://my-sso-base-url.awsapps.com/start
    sso_region = eu-central-1
    sso_role_name = AdministratorAccess
    region = us-west-1

    [profile company-audit]
    sso_account_id = 4XXXXXXXXXXX
    sso_start_url = https://my-sso-base-url.awsapps.com/start
    sso_region = eu-central-1
    sso_role_name = AdministratorAccess
    region = us-east-1

For Credential, MfA or Cross-Account accounts, ensure you have an appropriate block in ~/.aws/credentials

    [company-central]
    aws_access_key_id = AKXXXXXXXXXXXXXXXXXX
    aws_secret_access_key = 0000000000000000000000000000000000000000

    [company-staging]
    source_profile = company-central
    role_arn = arn:aws:iam::XXXXXXXXXXXX:role/Administrator
    region = ap-southeast-1
    
    [company-production]
    source_profile = company-central
    role_arn = arn:aws:iam::XXXXXXXXXXXX:role/Administrator
    mfa_serial = arn:aws:iam::YYYYYYYYYYYY:mfa/user.name
    region = us-east-1

## Ok, but now what?

Now we get to the fun part - doing stuff.

awsudobulk -u 'company-\*' aws s3 ls

Would for each account in the above config/credentials file do:
- For the SSO accounts, pull your temporary credentials through SSO after you've authenticated the application to AWS and your organization.
- For company-central, simply use the keys that are in the credentials file.
- For company-production, it would use your MFA to sts:GetSessionToken for company-central, and then use those temporary credentials to iam:AssumeRole into company-productoin.
- For company-staging, it would use your company-central keys to sts:GetSessionToken, and use those temporary credentials to iam:AssumeRule into company-staging.

More importantly (perhaps), it will (optionally) cache this data along the way, refreshing only as needed. So if you have a series of bulk commands to run, you don't need to sit around typing your MFA every minute or two, or perform SSO authentication for every command.  awsudobulk attempts to intelligently cache credentials in memory (and if permitted, on disk) to allow you to keep working until credentials expire (or are about to expire).

## But... why?

I tend to do a lot of freelance SRE/DevOps work with AWS, with the advent of ControlTower and other techniques, being able to do bulk operations quickly comes in handy when the UI doesn't really allow multiple simultanious logins.

## Can it do....?

Maybe?  It's very much written for a user-base of one.  Pull requests are welcome, but I can't promise to add them.  It fits my mental model, it may not fit yours.

Pull requests are welcome, as are forks.

## How does it work?

First, it takes an ordered approach to processing the accounts it can discover, and uses glob matching (NB: There may be merit to moving to regular expressions here, but for now, it's glob) to determine which ones the request applies to.

Second it takes one of two major logical paths:

- If the account is configured through SSO (e.g.: sso\_ variables in a 'profile XXX' block in ~/.aws/config), use AWS SSO credentials to authenticate the account, refreshing as needed.
- If the account is configured through credentials (e.g., in ~/.aws/credentials), then, in order:

   -   Initialize base credentials based on access-keys (recursing through source_profile if needed)
   -   If an MFA is defined, use STS to fetch a token (sts:GetSessionToken) using the user-provided MfA token.
   -   If a role account is defined, use whatever credentials are set (either straight access/secret, or the derived session from MfA) to assume the role account to call sts:AssumeRole.
   -   Use the resultant credentials to call the command in the arguments with appropriate environment variables set


## Notes:

*   awsudobulk if a region is explicitly set in the account, it will override the run-time environment (this may change, subject to debate)
*   External environment is preserved excepting AWS_ACCESS_KEY_ID/AWS_SECRET_ACCESS_KEY/AWS_DEFAULT_REGION/AWS_SESSION_TOKEN which may get overridden on a per-execution basis.  This also means that if you set variables such as AWS_PROFILE, the results may be undefined.
*   Credentials are obtained first, then the commands are run serially.  This is designed to minimize and front-load operator attention, but if you have especially short timeouts on sessions and long-running commands, there may be issues.
*   In that same vein, SSO accounts are processed before Credential accounts - this is an implementation detail, but you may wish to be aware.
*   Profile processing may differ from awscli - this is an implementation detail - awsudobulk is largely compatable but the parser and implementation are independant at this time, so if you encounter incompatabilities, please report an issue.

## Troubleshooting

Run awsudobulk with

    -stderrthreshold INFO -logtostderr -v 9

As parameters - if that doesn't clarify, send the output from that, and if possible your REDACTED ~/.aws/config and ~/.aws/credentials in an issue and I'll happily take a look.

