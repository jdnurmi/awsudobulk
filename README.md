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


