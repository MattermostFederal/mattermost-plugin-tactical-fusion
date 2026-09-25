# PR preview environments on AWS (`preview` label)

## Overview

Adding the GitHub label `preview` to a pull request stands up a throwaway
Mattermost server on AWS with that PR's plugin bundle installed, reachable at
`https://<app>-pr<N>.<preview-domain>` (for this repo,
`https://tactical-fusion-pr49.mattermostfed-preview.com`), and posts the URL as a PR
comment. Pushing more commits redeploys the bundle onto the same server.
Removing the label or closing the PR tears it down.

The pattern is shared across MattermostFederal plugin repos: each repo carries
one copied workflow file, and everything else lives in a private `pr-preview`
repo plus a dedicated AWS account.

## Problem statement

Reviewers today have to build a PR locally (`make docker-setup && make deploy`)
to try it, and nobody outside engineering can try it at all. A hosted preview
lets a reviewer, a product owner, or a customer contact click a link in the PR
and use the plugin against a real Enterprise server.

## Threat model

A preview runs code chosen by whoever authored the PR, including external
contributors on public repos. The design assumes the plugin bundle is hostile,
the preview instance is compromised the moment the bundle is installed, and
the goal is to make sure a compromised preview yields nothing beyond itself:
no secrets, no other previews, no private source, no foothold in the org's
GitHub or AWS, and no way to attack the org's production hostnames.

## Decisions

- **Packaging**: new **private** repo `MattermostFederal/pr-preview` holds the
  scripts, instance files, reaper, and the OpenTofu for the preview account.
  Plugin repos are public and GitHub only lets a public repo `uses:` reusable
  workflows and actions from public repos, so each plugin repo gets a thin
  `.github/workflows/preview.yml` that checks out `pr-preview` at runtime with a
  GitHub App installation token and runs its scripts.
- **No PR code runs in the preview workflow.** Every plugin repo's `pr.yml`
  already builds `dist/*.tar.gz` under the `pull_request` event and uploads it
  as the `plugin-dist` artifact. The preview workflow downloads that artifact
  for the PR's head SHA. A `pull_request_target` job that ran PR code would
  write to the `main` branch cache scope, which `release.yml` restores while
  holding the plugin signing key, so that path is ruled out.
- **Separate registrable domain.** Previews live under a dedicated domain
  (`mattermostfed-preview.com`, or whatever is available) registered in the
  `mfi-preview` account, not under `mattermostfed.com`. `chat.` and `hub.`
  already exist there; a same-site preview running fork code could set
  `Domain=.mattermostfed.com` cookies, sit inside those apps' SameSite CSRF
  boundary, share Let's Encrypt's 50 certificates per week per registered
  domain, and leave dangling org-domain records. The `route53.tf` change to
  `terraform-aws-organization` is therefore not needed.
- **AWS account**: new commercial member account `mfi-preview` under the `dev`
  OU of the **MattermostFederal AWS Organization** (the commercial
  organization managed by `terraform-aws-organization`, management account
  `060212350641`), created in `commercial/accounts.tf`. Region `us-east-1`.
  Nothing in this plan touches any personal AWS account; see "AWS account
  boundary" below.
- **AWS auth**: GitHub OIDC, trusting only `mattermost-plugin-*` repos and the
  `pr-preview` main branch. No static keys.
- **Server**: stock `mattermost/mattermost-enterprise-edition` plus
  `postgres:14-alpine` with the environment from `docker-compose.dev.yml`, and
  Caddy in front for automatic Let's Encrypt HTTPS. No ALB, no ACM, no license.
- **Access**: previews are open servers. Anyone with the link creates an
  account and joins team `test`, which is public. The box is disposable and
  lives at most seven days, so open sign-up costs nothing worth protecting.
- **Admin**: username `admin`; the password is unique per preview, derived as
  `HMAC-SHA256(PREVIEW_ADMIN_SECRET, FQDN)` encoded as 24 alphanumeric
  characters. The key is stored twice: as the org secret the workflow uses,
  and in Secrets Manager in `mfi-preview` as `pr-preview/admin-secret`, which
  `scripts/preview password <fqdn>` reads so anyone with Identity Center
  access to the account can derive a password. The instance role has no
  Secrets Manager permission. The `ready` comment also carries the password
  encrypted with `age` to the GitHub SSH keys (`github.com/<login>.keys`,
  ed25519 and RSA) of the person who added the label and the PR author, so
  neither needs AWS access or a shared secret; users without published keys
  fall back to the helper. It is never posted, never in the
  Mattermost container's environment, and a captured one opens one preview
  only. Team `test`.
- **Bundle delivery**: the workflow uploads the bundle to S3 under a random
  per-preview prefix and the instance installs it with `mmctl --local plugin
  add ... --force` then `plugin enable`, exactly as `make docker-deploy` does.
  Redeploys run on the box through SSM Run Command. No SSH, no key pairs.
- **Label approves a commit, not a branch.** For a fork PR the approved
  commit is the head SHA in the `labeled` event payload, and the first
  deployment happens only when that SHA still equals the live head and the
  fetched bundle. Any later fork push tears the preview down and removes the
  label; a maintainer re-adds it to preview the new commit. Pushes from
  branches in the org repo redeploy directly.
- **Every run reconciles.** Each workflow run reads the PR's live state and
  converges to it (up when open and labeled, down otherwise) instead of
  trusting the event payload, so a displaced or out-of-order run cannot leave
  a preview standing.

## AWS account boundary

Every AWS resource in this plan lives in the MattermostFederal AWS
Organization, in the `mfi-preview` member account. No resource, domain
registration, state bucket, IAM role, or credential is created in, or read
from, a personal AWS account.

- **Credentials for one-time steps** come from the org's Identity Center or
  the management account, assuming `OrganizationAccountAccessRole` in
  `mfi-preview`. The local `default` AWS CLI profile is never used. Every
  manual command in this plan runs with an explicit profile, for example
  `[profile mfi-preview]` with `role_arn` pointing at that role and
  `source_profile` pointing at an org identity.
- **Guard before every mutating command**: `aws sts get-caller-identity
  --query Account --output text` must print the `mfi-preview` account id
  from the `terraform-aws-organization` output. `bootstrap/README.md` puts
  this check first, and `bootstrap/bootstrap.yaml` is deployed with
  `--profile mfi-preview`. The `pr-preview` scripts run only in GitHub
  Actions through OIDC, so they cannot pick up a local profile.
- **Domain registration** (`aws route53domains register-domain`) runs in
  `mfi-preview` with the same profile, so the domain, its hosted zone, and
  its renewal billing belong to the org, not to a person. Registrant contact
  is the org's IT contact (`it@mattermostfed.com`), not an individual.
- **State and CI** stay in the account: the tofu backend bucket is
  `<mfi-preview-acct>-tf-state` in `mfi-preview`, the OIDC provider and both
  role sets are in `mfi-preview`, and `PREVIEW_AWS_ROLE_ARN` carries the
  `mfi-preview` account id.
- The org already has a `corey-hulen` dev account in `accounts.tf`. It is not
  used for previews; the plan creates `mfi-preview` so preview workloads have
  their own blast radius and cost line.

## Out of scope

- Enterprise license on previews. The plugin process can read the server's
  environment and database, so a license on a box running fork code is
  exposed.
- Wildcard or DNS-01 certificates, Docker Hub pull tokens, egress
  restrictions beyond what is listed.
- Per-repo post-install configuration such as map packages (`make
  docker-packages`); those are local build output and never reach the box.

## Architecture

```
plugin repo PR labeled "preview"
  .github/workflows/preview.yml  (pull_request_target: labeled, unlabeled, synchronize, closed)
    fetch     : no secrets, actions:read only; wait for pr.yml on head SHA, download and validate
                plugin-dist, re-upload as preview-bundle
    reconcile : checkout pr-preview@refs/tags/v1 via checkout App token, OIDC -> GithubActionsPreview,
                read live PR state, scripts/preview reconcile (up or down)

mfi-preview account (us-east-1)
  Route53 zone <preview-domain>            A <app>-pr<N> -> instance public IP (TTL 60)
  S3 bucket mfi-preview-bundles-<acct>     previews/<host>-<nonce>/{bundle.tar.gz,env,instance.tar.gz}, 14d expiry
  VPC 10.90.0.0/24, one public subnet, SG 80/443 in, no 22
  EC2 t3.large AL2023, standard CPU credits, 30 GB gp3, IMDSv2 hop limit 1, instance profile (SSM agent + S3 get)
    docker compose: postgres:14-alpine, mattermost-enterprise-edition:<tag>, caddy:2
  IAM: GithubActionsPreview (narrow OIDC trust, least privilege); preview-tofu-ro/rw (bootstrap)
  AWS Budget alarm on the account
```

## Repos and files

Only this repo is checked out locally. The other two are cloned into a
scratch directory for their PRs. Work here happens in the `pr-preview`
worktree on branch `ci/pr-preview`, never in `main/`.

### 1. `terraform-aws-organization` (one PR)

`commercial/accounts.tf`: add `aws_organizations_account.mfi_preview`
(`name = "mfi-preview"`, `email = "it+mfi-preview@mattermostfed.com"`,
`parent_id = aws_organizations_organizational_unit.dev.id`) and an output
`mfi_preview_account_id` in `commercial/outputs.tf`. No `route53.tf` change.

### 2. `terraform-github-management`

`stacks/repos/repos_management.tf`: declare `"pr-preview"` with
`visibility = "private"`, teams `admins = "admin"` and
`infrastructure = "admin"`, `branch_protection = "federal-management"`
(required review, CODEOWNERS), `bp_branch_pattern = "main"`,
`delete_branch_on_merge = true`. Add the `bp_status_checks` entry
`{ context = "Plan (infra)", integration_id = 15368 }` in a second PR after
the first content is pushed. Add rulesets: tags `v*` may be created, moved, or
deleted only by the `admins` team; branches named `v*` may not be created.
Whoever can move `v1` controls every plugin repo's trusted job, so this is
load-bearing. Labels are not managed there; the `preview` label is created
per repo with `gh label create`.

### 3. `pr-preview` (new, private)

```
README.md                        adopt-in-3-steps guide, env contract, operations notes, approver rules
.github/workflows/tofu_pr.yml    plan infra/ on PR       (mirrors the org repo's tofu_pr.yml, ROLE_TO_ASSUME_RO)
.github/workflows/tofu_apply.yml apply on push to main  (mirrors the org repo's tofu_apply.yml, ROLE_TO_ASSUME_RW)
.github/workflows/reaper.yml     hourly cron plus workflow_dispatch
bootstrap/bootstrap.yaml         CloudFormation: tf-state bucket, GitHub OIDC provider, preview-tofu-ro/rw roles
bootstrap/README.md              exact aws cli commands, run once via OrganizationAccountAccessRole
infra/                           tofu root: versions, backend (key pr-preview/infra.tfstate), providers,
                                 network.tf route53.tf s3.tf iam_instance.tf iam_github.tf budget.tf outputs.tf
config.env                       committed non-secret ids from the tofu outputs: BUCKET, SUBNET_ID, SG_ID,
                                 INSTANCE_PROFILE, ZONE_ID, DOMAIN, AWS_REGION, MM_IMAGE_TAG, MAX_PREVIEWS
scripts/preview                  CLI: reconcile | down | reap | password; up, deploy, comment are functions
instance/user-data.sh            about 1.5 KB, rendered with envsubst (BUCKET, PREFIX only)
instance/setup.sh                first boot: compose up, admin and team, shred env, wait for DNS, caddy, deploy.sh, READY
instance/deploy.sh               idempotent (re)install of /opt/preview/bundles/bundle.tar.gz
instance/docker-compose.yml      postgres, mattermost, caddy
instance/Caddyfile               {$FQDN} { reverse_proxy mattermost:8065 }
templates/preview.yml            canonical thin workflow to copy into each plugin repo
```

`bootstrap/bootstrap.yaml` is the org repo's `github_actions_oidc_role.yaml`
(OIDC provider, a read-only role trusting
`repo:MattermostFederal@233000902/pr-preview@<repo-id>:pull_request`, a
read-write role with `AdministratorAccess` trusting the same immutable subject
at `ref:refs/heads/main`) plus one `AWS::S3::Bucket` named `<acct>-tf-state`.
The repository id is a stack parameter, so bootstrap runs after
`terraform-github-management` has created the repo. The read-only role also needs
`s3:PutObject` and `s3:DeleteObject` on the state bucket for the lockfile.

`config.env` is the single source of every non-secret id. Nothing is repeated
in org variables. It is updated once after each tofu apply that changes an
output.

### 4. Each plugin repo, this one first

Add `.github/workflows/preview.yml`, a verbatim copy of `templates/preview.yml`
with every `uses:` pinned to a full SHA plus a `# vX.Y.Z` comment, each SHA
checked to be reachable from an upstream tag. Nothing else changes. Adoption
requirements, stated in the README: a `pr.yml` that runs on `pull_request`
and uploads exactly one `dist/*.tar.gz` as the `plugin-dist` artifact, laid
out as `<plugin-id>/plugin.json` at the tarball root (the starter Makefile's
`tar -cvzf ... $(PLUGIN_ID)`). Every current MattermostFederal plugin repo
already meets both. The README also states who counts as an approver: anyone
with triage, write, maintain, or admin on the repo, including teams and custom
org roles that grant those.

## Tofu resources (`pr-preview/infra/`)

| File | Resources |
|---|---|
| `network.tf` | `aws_vpc.preview` 10.90.0.0/24, `aws_subnet.public` with `map_public_ip_on_launch`, internet gateway, route table with default route, `aws_security_group.preview_instance` (80 and 443 from 0.0.0.0/0 and ::/0, egress all, no self-referencing rule) |
| `route53.tf` | `data.aws_route53_zone.preview` for the registered domain's zone (registration itself is a one-time `aws route53domains register-domain`, which creates the zone) |
| `s3.tf` | `aws_s3_bucket.bundles` `mfi-preview-bundles-<acct>`, public access block, SSE-S3, bucket policy denying non-TLS, lifecycle expiring `previews/` after 14 days and aborting multipart uploads after 1 day |
| `iam_instance.tf` | role `preview-instance` trusting ec2 with an inline SSM-agent-only policy (the `ssm:UpdateInstanceInformation`, `ssmmessages:*`, `ec2messages:*`, `ssm:ListAssociations`, `ssm:ListInstanceAssociations`, `ssm:PutInventory`, `ssm:UpdateInstanceAssociationStatus` set, no `ssm:GetParameter*`, no `ssm:GetDocument`), plus `s3:GetObject` on `previews/*` and nothing else on S3 (no `ListBucket`); instance profile |
| `iam_github.tf` | `data.aws_iam_openid_connect_provider.github`; role `GithubActionsPreview` trusting `aud = sts.amazonaws.com` and `StringLike sub` in `["repo:MattermostFederal/mattermost-plugin-*:*", "repo:MattermostFederal@233000902/mattermost-plugin-*@*:*", "repo:MattermostFederal@233000902/pr-preview@<repo-id>:ref:refs/heads/main"]`, 1 h max session, with the policy below. Repositories created after July 15, 2026 present immutable subjects that carry the owner and repository ids (`repo:OWNER@OWNER-ID/REPO@REPO-ID:...`); `pr-preview` is one, and the second plugin pattern covers plugin repos created later. The repo id lives in `infra/terraform.tfvars`. |
| `budget.tf` | `aws_budgets_budget` monthly cost budget with an email alert at 80 and 100 percent |
| `outputs.tf` | `zone_id`, `domain`, `bucket`, `subnet_id`, `security_group_id`, `instance_profile`, `github_role_arn`, `account_id` |

`GithubActionsPreview` policy statements:

1. `ec2:RunInstances` on subnet `<subnet>`, security group `<sg>`, network interface, and volume ARNs, unconditional; on `image/*` with `ec2:Owner = amazon`.
2. `ec2:RunInstances` on `instance/*` with `aws:RequestTag/preview:managed = "true"` and `ec2:InstanceType = t3.large`.
3. `ec2:CreateTags` on instances and volumes when `ec2:CreateAction = RunInstances`, and on instances tagged `preview:managed = true` so `preview:state` and `preview:sha` can be set.
4. `ec2:TerminateInstances` on instances tagged `preview:managed = true`.
5. `ec2:DescribeInstances` and `ec2:DescribeInstanceStatus` on `*`.
6. `iam:PassRole` on `role/preview-instance` with `iam:PassedToService = ec2.amazonaws.com`.
7. `ssm:GetParameter` and `ssm:GetParameters` on `/aws/service/ami-amazon-linux-latest/*`.
8. `route53:ChangeResourceRecordSets` on the zone, restricted to record type `A`; `ListResourceRecordSets` on the zone; `GetChange` on `change/*`.
9. `s3:PutObject`, `s3:GetObject`, `s3:DeleteObject` on `<bucket>/previews/*`; `s3:ListBucket` with prefix `previews/*`.
10. `ssm:SendCommand` on `document/AWS-RunShellScript` and on instances tagged `preview:managed = true`; `GetCommandInvocation`, `ListCommandInvocations`, `DescribeInstanceInformation` on `*`.

Blast radius of the role, accepted and documented: any workflow in a
`mattermost-plugin-*` repo can assume it, so anyone with write on such a repo
can create, terminate, or re-tag previews, read bundles, and point records in
the preview zone anywhere. The preview zone is isolated from
`mattermostfed.com`, so none of that reaches production. Repo creation in the
org is managed by `terraform-github-management`, so throwaway
`mattermost-plugin-*` repos are not a casual option.

## Workflow design

### Trigger and trust boundary

`pull_request_target` with `types: [labeled, unlabeled, synchronize, closed]`.
It fires for fork PRs on public repos with secrets and `id-token` available,
runs the default branch's copy of the workflow so a PR cannot edit its own
preview workflow, and behaves the same on the org's private plugin repos. The
workflow never checks out the PR and never runs PR code. The `preview` label
is the approval; removing it or closing the PR tears down. Secrets are
referenced only inside job steps behind a job-level `if:`, never in
workflow-level `env:`. Runs whose actor is `dependabot[bot]` are skipped,
since they receive no secrets.

Two jobs:

- **fetch**: `permissions: actions: read`, no secrets, no checkout, no
  concurrency group. Runs when the PR is open and labeled (checked live with
  `gh api`, not from the payload). Waits up to 30 minutes for the `pr.yml`
  run on the head SHA, downloads and validates the bundle (contract below),
  and uploads it as the `preview-bundle` artifact of this run. A bundle is
  missing when a first-time contributor's `pr.yml` run is awaiting approval,
  when the run failed, or when its artifact passed the seven-day retention.
  The failure comment says so and tells the maintainer to re-run PR Validation
  for the head commit, then remove and re-add the label. The 30-minute wait happens here so the AWS session in the
  next job is never near its 1-hour limit.
- **reconcile**: `needs: fetch` (with `if: always()` so teardowns run when
  fetch is skipped), `permissions: id-token: write, pull-requests: write,
  actions: read`, job-level `concurrency: group:
  ${{ github.workflow }}-preview-${{ github.event.pull_request.number }}`,
  `cancel-in-progress: false`. Mints the checkout App token
  (`repositories: pr-preview`), checks out `MattermostFederal/pr-preview` at
  `ref: refs/tags/v1` with `persist-credentials: false`, assumes
  `vars.PREVIEW_AWS_ROLE_ARN`, sets `GH_REPO` to the plugin repo, downloads
  `preview-bundle` if present, and runs `scripts/preview reconcile`.

Why reconcile rather than act on the payload: a concurrency group holds one
running and one pending run and a new run replaces the pending one. Acting on
payloads, one push after a label removal would cancel the queued teardown and
leave the preview up until the reaper. With reconciliation the last run
always converges to the PR's actual state, so displacement is harmless.

Tokens: `github.token` is used for the PR comment, the label, and the
artifact download. The checkout App token is used only to check out
`pr-preview`. The reaper uses a second App.

### Two GitHub Apps

An org secret visible to public repos can be read by anyone with write on any
of those repos. So the key that must be an org secret is kept nearly
powerless, and the powerful key is not an org secret.

- `mmf-preview-checkout`: Contents read, Metadata read, installed only on
  `pr-preview`. Its key is the org secret `PREVIEW_CHECKOUT_APP_PRIVATE_KEY`
  with variable `PREVIEW_CHECKOUT_APP_ID`. If leaked, it reads the
  `pr-preview` source, which holds no secrets.
- `mmf-preview-reaper`: Pull requests write, Metadata read, installed on every
  plugin repo. Its key is a `pr-preview` repo secret used only by
  `reaper.yml` on `main`.

### Thin per-repo workflow (`templates/preview.yml`)

About 80 lines. The only per-repo differences are two optional `env` lines:
`PREVIEW_APP` (default: repo name with `mattermost-plugin-` stripped,
lowercased, sanitized to `[a-z0-9-]`, truncated so `<app>-pr<N>` fits 63
characters) and `PREVIEW_MM_IMAGE_TAG` (default: the constant in
`config.env`, full semver such as `11.8.0`). Environment passed to the
scripts: `REPO`, `PR_NUMBER`, `EVENT_ACTION`, `EVENT_HEAD_SHA`, `GH_TOKEN`,
`RUN_URL`, `BUNDLE_DIR`, `FETCH_ERROR`, `PREVIEW_ADMIN_SECRET`, plus the two
optional overrides. PR event fields reach `run:` only through
`env:`, never inline. `refs/tags/v1` is a named exception to the
no-floating-refs rule because it is a `checkout` with a token, not a `uses:`,
and the tag is protected by ruleset.

### Bundle intake (fetch job)

- `gh run list -R "$REPO" --workflow pr.yml --event pull_request --commit
  "$SHA" --json databaseId,status,conclusion,headRepository` (never
  `displayTitle`); require `headRepository` to equal the PR's head repo; take
  the newest completed run; poll until completed or 30 minutes.
- List the run's artifacts through the API, require exactly one named
  `plugin-dist`, and refuse if `size_in_bytes` exceeds 250 MB (tactical-fusion
  is about 77 MB).
- Download the raw zip with `gh api .../artifacts/<id>/zip` into an empty
  `$RUNNER_TEMP/intake`. `zipinfo -1` must list exactly one entry matching
  `^[A-Za-z0-9._-]+\.tar\.gz$`. `unzip -p` that entry to
  `$RUNNER_TEMP/bundle.tar.gz`. `gh`'s own extractor is not used.
- `timeout 60 tar -tvzf bundle.tar.gz | head -c 1M`: every member must be a
  regular file or directory (no symlinks, hardlinks, devices) under a single
  top-level directory. `timeout 60 tar -xzOf bundle.tar.gz --occurrence=1
  "<dir>/plugin.json" | head -c 65536 | jq`: `id` must equal `<dir>` and match
  `^[A-Za-z0-9._-]{3,190}$`; `version` must match a semver pattern. Abort on
  any mismatch. Every expansion is quoted; nothing attacker-influenced is
  echoed unvalidated.
- Upload `bundle.tar.gz` plus a `manifest.env` (id, version) as the
  `preview-bundle` artifact with `retention-days: 1`.

### `scripts/preview` contract

- Sources `config.env`. Derived: `HOST=${PREVIEW_APP}-pr${PR_NUMBER}`,
  `FQDN=${HOST}.${DOMAIN}`, `URL=https://${FQDN}`,
  `ADMIN_PASSWORD=$(password "$FQDN")`. Constants: instance type `t3.large`,
  `CpuCredits=standard`, root 30 GB gp3, readiness timeout 900 s.
- `reconcile`: read `gh api repos/$REPO/pulls/$PR_NUMBER` for state, labels,
  `head.sha`, `head.repo.full_name`. Then:
  - not open, or label absent: `down`.
  - open and labeled, org branch: `up` with the live head SHA.
  - open and labeled, fork: if an instance exists with `preview:sha` equal to
    the live head, nothing to do; if it exists with a different SHA, `down
    fork-push` (tears down, removes the label, comments); if none exists,
    `up` only when the run is the `labeled` event and the payload's head SHA
    (`EVENT_HEAD_SHA`) equals the live head, recording it in `preview:sha`;
    otherwise the fork moved since the label and the label is removed with
    the fork-push comment.
  - `up` refuses when the count of live managed instances is at
    `MAX_PREVIEWS` (10) and comments accordingly.
- `find_instance`: `describe-instances` filtered on `tag:preview:host`,
  `tag:preview:repo`, `tag:preview:pr`, and `instance-state-name` in
  `pending,running`. A running instance whose repo or PR tags differ from the
  request is an error, and `down` deletes DNS and S3 only when the tags match.
  An instance tagged `preview:state=failed` is terminated before a new one is
  created.
- `up`: if an instance is found, fall through to deploy. Otherwise generate a
  128-bit `NONCE`, `PREFIX=previews/${HOST}-${NONCE}`, upload `env`
  (FQDN, MM_IMAGE_TAG, ADMIN_PASSWORD), `instance.tar.gz`, and `bundle.tar.gz`
  under it; `run-instances` with
  `--image-id resolve:ssm:/aws/service/ami-amazon-linux-latest/al2023-ami-kernel-default-x86_64`,
  IMDSv2 required, hop limit 1, `--credit-specification CpuCredits=standard`,
  30 GB gp3, tags including `preview:prefix`; poll for the public IP; UPSERT
  the A record and wait for INSYNC; `comment starting`; wait for readiness;
  delete the `env` object from S3; `comment ready`. On timeout: delete the
  `env` object, tag `preview:state=failed`, `comment failed`, keep the
  instance for `ssm start-session`.
- deploy (function): upload the bundle to the instance's prefix; `ssm_run`
  with the fixed command string `/opt/preview/deploy.sh`; the invocation's exit
  status is the readiness signal; set `preview:sha`; `comment redeployed`.
  Redeploy replaces only the bundle; instance-file changes reach existing
  previews only by re-labeling.
- `down`: delete the A record first, then terminate matching instances, then
  `s3 rm --recursive` on the instance's prefix, then `comment down`. With
  `--reason fork-push`, also remove the label and use the fork-push comment
  text. Every step tolerates "already gone".
- `reap`: list instances tagged `preview:managed=true` in `pending,running`;
  validate tags (`repo` against `^MattermostFederal/[A-Za-z0-9._-]+$`, `pr`
  against `^[1-9][0-9]*$`, `host` against `^[a-z0-9-]{1,63}$`, and `host` must
  equal the value recomputed from repo and PR); skip and report an instance
  whose tags fail; per-instance error handling so one bad instance never stops
  the loop. For each, `gh api repos/<repo>/pulls/<pr>` for state and labels;
  call `down` when the PR is closed, the label is absent, or the instance is
  older than 7 days. Then delete every A record in the zone whose value is
  not the public IP of a live managed instance. No age test: Route53 records
  carry no timestamp, and a stale record is the takeover window. Summary to
  `$GITHUB_STEP_SUMMARY`.
- `password <fqdn>`: prints the derived password for maintainers who hold
  `PREVIEW_ADMIN_SECRET`.
- Output hygiene: SSM output is truncated to 4 KB, stripped of control
  characters, and printed inside `::stop-commands::<random>`. Only the exit
  code is trusted. Nothing from the bundle or the instance enters the comment.
- Tags: `Name`, `preview:managed=true`, `preview:repo`, `preview:pr`,
  `preview:host`, `preview:prefix`, `preview:sha`, `preview:state` (set only
  on failure).

### PR comment

Find by the marker `<!-- pr-preview -->` AND author in
`github-actions[bot]`, `mmf-preview-reaper[bot]`, paging with `--paginate`;
create if absent. Anyone can post the marker on a public repo, so author
matching stops the bot from editing an attacker's comment. The body is a
small table with URL, status (starting, ready, redeploying, failed, torn
down), the deployed short commit, and updated time linking to the run, then:
"Login `admin`; maintainers get the password with `scripts/preview password
<fqdn>`; team `test`. Remove the `preview` label to tear down. Previews are
reaped when the PR closes or after 7 days." The `failed` state adds the
instance id and the log path `/var/log/preview-userdata.log`, and notes that
a first-time contributor's PR Validation run must be approved first. The
fork-push state says the preview was torn down because the fork pushed new
commits and a maintainer can re-add the label after reviewing the new commit.

### Reaper

`reaper.yml`: hourly cron plus `workflow_dispatch`, reaper App token with no
repo filter, assumes `GithubActionsPreview`, runs `scripts/preview reap`.

## Instance files

- `user-data.sh`: `set -euo pipefail` (no `-x`), log to
  `/var/log/preview-userdata.log` only, never the console.
  `dnf install -y docker jq bind-utils`, enable docker, install the compose v2
  binary (pinned, sha256 checked), add
  `iptables -I DOCKER-USER -d 169.254.169.254 -j DROP` so containers cannot
  reach IMDS even if a hop limit is ever raised, retry-loop `aws s3 cp` of
  `instance.tar.gz`, `env` (mode 600), and `bundle.tar.gz` from `$PREFIX`
  into `/opt/preview`, then `exec /opt/preview/setup.sh`.
- `setup.sh` ports `make docker-setup`: `set -euo pipefail`, no xtrace
  anywhere near the password. Source `/opt/preview/env`, write
  `/opt/preview/.env` with only FQDN and MM_IMAGE_TAG, `compose --env-file
  .env up -d postgres mattermost`, wait on `localhost:8065/api/v4/system/ping`,
  `mmctl --local user create --email admin@example.com --username admin
  --password-file <(printf %s "$ADMIN_PASSWORD") --system-admin` (or stdin;
  never argv), `team create test`, `team users add test admin`, then `shred
  -u /opt/preview/env` and unset the variable. Wait until an authoritative
  name server for the zone answers `dig +short $FQDN` with the instance's own
  IP from IMDSv2. Ordering: admin and team, `shred`, `touch SETUP_DONE`,
  `deploy.sh`, DNS wait, `compose up -d caddy`, `touch READY`. Caddy starts
  only after the plugin is installed, so an HTTPS ping from the runner means
  the whole first boot succeeded.
- `deploy.sh` ports `make docker-deploy`: fail fast if `SETUP_DONE` is
  missing (it exists before the first-boot deploy; `READY` is written only
  after Caddy is up);
  pull the bundle from the prefix; derive and validate id and version with
  the same rules as the runner; `compose stop mattermost`; remove the
  `plugins`, `client-plugins`, and `config` named volumes so nothing a
  previous commit planted survives; `compose up -d --force-recreate
  mattermost`; wait for ping; `compose exec -T mattermost mmctl --local
  plugin add /bundles/bundle.tar.gz --force`; `plugin enable "$id"`; verify
  with `mmctl --local plugin list --json` that the active set is exactly the
  prepackaged allowlist plus `$id` at `$version`; exit non-zero otherwise.
  `data` and Postgres persist so reviewers keep their test content.
- `docker-compose.yml`: the `docker-compose.dev.yml` environment verbatim
  except `MM_SERVICESETTINGS_SITEURL=https://${FQDN}`, no host port for
  Mattermost, no `env_file:` on the Mattermost service, `plugins`,
  `client-plugins`, and `config` as named volumes, `./bundles:/bundles:ro`.
  On the Mattermost service: `security_opt: [no-new-privileges:true]`,
  `pids_limit: 512`, `mem_limit: 4g`. On every service: default bridge
  network, no `network_mode: host`, no `privileged`, no docker socket mount,
  `restart: unless-stopped`. `caddy:2` on 80 and 443 with `/data` persisted.
- `Caddyfile`: `{$FQDN} { reverse_proxy mattermost:8065 }`.

## Sequencing and one-time setup

1. terraform-aws-organization PR (account), apply, note the account id.
2. After step 3 has created the `pr-preview` repository (its id is a stack
   parameter), bootstrap in `mfi-preview` from a local clone of pr-preview,
   using an org identity that assumes `OrganizationAccountAccessRole` there. First confirm
   the account: `aws --profile mfi-preview sts get-caller-identity --query
   Account --output text` must equal the id from step 1; stop if it does not.
   Then
   `aws --profile mfi-preview cloudformation deploy --stack-name pr-preview-bootstrap --template-file bootstrap/bootstrap.yaml --capabilities CAPABILITY_NAMED_IAM`.
   Register the preview domain in the same account with `aws --profile
   mfi-preview route53domains register-domain` (or the console while signed
   in to `mfi-preview`), registrant `it@mattermostfed.com`, which creates its
   hosted zone.
3. terraform-github-management PR declaring `pr-preview` with the tag and
   branch rulesets and without the required check, apply, push the repo
   contents; read the repo id with `gh api repos/MattermostFederal/pr-preview
   --jq .id` and put it in `infra/terraform.tfvars`; set repo secrets
   `ROLE_TO_ASSUME_RO`, `ROLE_TO_ASSUME_RW`, `PREVIEW_REAPER_APP_PRIVATE_KEY`
   and variables `ACCOUNT_ID`, `PREVIEW_REAPER_APP_ID`; then a follow-up PR
   adding the `Plan (infra)` required check. The `infrastructure` team gets
   `maintain`, not `admin`, because repository admins can edit the rulesets
   that protect `v*`.
4. pr-preview infra PR, plan, merge, apply; copy the outputs into
   `config.env` in a second PR.
5. GitHub Apps: `mmf-preview-checkout` (Contents read, Metadata read,
   installed on pr-preview only) and `mmf-preview-reaper` (Pull requests
   write, Metadata read, installed on every plugin repo). Org variable
   `PREVIEW_CHECKOUT_APP_ID`, org secret `PREVIEW_CHECKOUT_APP_PRIVATE_KEY`.
6. Org variable `PREVIEW_AWS_ROLE_ARN`; org secret `PREVIEW_ADMIN_SECRET` (32
   or more random characters). Visibility must include public repos. Steps 5
   and 6 need an org admin.
7. Tag pr-preview `v1`. `v1` is a moving major tag guarded by the ruleset:
   the env contract only gains fields within `v1`; a breaking change means
   `v2` and editing every copied workflow.
8. Org Actions event policy: GitHub disables `pull_request_target` by default
   on public repositories from November 2, 2026 unless an explicit enterprise,
   organization, or repository event policy allows it (workflow execution
   protections, generally available September 17, 2026). Configure an
   organization policy that allows `pull_request_target` for the plugin repos
   before adopting. The repos' action allowlist already permits GitHub-owned
   and verified-creator actions plus `opentofu/setup-opentofu`, which covers
   every action the workflow uses.
9. Per plugin repo: `gh label create preview --color 0E8A16 --description "Deploy a preview environment for this PR"`,
   copy `templates/preview.yml`, merge, label a PR.

## Security notes

- No PR code runs in the workflow. The bundle is treated as hostile data and
  validated in a job with no secrets before any trusted job sees it.
- A compromised preview yields only itself: its own admin password (derived
  per host), its own bundle (found by nonce, since the instance role cannot
  list the bucket), its own certificate, and an AWS instance role that can
  only talk to the SSM agent and fetch objects it already knows the name of.
  IMDS is blocked from containers twice (hop limit and iptables). The
  Postgres credentials are static and reach only that box.
- The org-wide App key can only read `pr-preview`. The key that can write to
  PRs never leaves `pr-preview`'s own secrets.
- Previews are on a separate registrable domain, so fork code can never set
  cookies for, sit same-site with, or consume certificate quota shared with
  `mattermostfed.com`.
- DNS is deleted before termination, and the hourly reaper deletes any record
  not backed by a live instance, so the dangling-record window is minutes.
- Redeploys wipe plugin and config volumes so a malicious earlier commit
  cannot persist under a benign later one.
- Accepted: certificate transparency logs and public DNS reveal
  `<app>-pr<N>` names, which leaks private repo names and PR activity.
  Anyone with write on a `mattermost-plugin-*` repo can assume the preview
  role and disrupt previews. Cost abuse is bounded by the label gate, the
  10-preview cap, standard CPU credits, the 7-day reaper, and the budget alarm.

## Gotchas already folded in

Route53's default negative TTL is 86400, so the name is never resolved before
INSYNC. Terminated instances stay visible for about an hour, so every lookup
filters on state. `gh` resolves the repo from the checkout's origin, so
`GH_REPO` is set explicitly. `mmctl --local` needs `exec -T` and
`EnableUploads=true`. SITEURL must be the https URL. Compose v2 and `dig` are
installed manually on AL2023. User-data runs once, so redeploys go through
SSM. `send-command` fails until the agent registers, so
`describe-instance-information` runs first. Let's Encrypt allows 5 duplicate
certificates per week per name and 50 per registered domain; the cap of 10
concurrent previews keeps churn inside that on the dedicated domain. Org
secrets must be visible to public repos. An Actions allowlist may need
`aws-actions/*` and `actions/create-github-app-token`. `pr.yml` runs tests
before `make dist`, so a PR that fails tests has no bundle and the preview
reports that. Secret masking is exact-string, so the derived password is
alphanumeric and never transformed before use.

## Verification

1. `tofu plan` is clean after apply and `config.env` matches the outputs. `aws sts get-caller-identity` in the tofu apply log, the bootstrap stack's account, the domain's registrant account, and the account id inside `PREVIEW_AWS_ROLE_ARN` all equal the `mfi-preview` id; `aws organizations list-accounts` from the management account lists `mfi-preview` under the `dev` OU.
2. Assume `GithubActionsPreview` from a `mattermost-plugin-*` repo workflow and confirm: `run-instances` without `preview:managed=true` or with `t3.xlarge` is denied and with the right tags and type succeeds; a TXT record change is denied; terminating an untagged instance is denied. Print the OIDC token claims once to confirm the `sub` shape for `pull_request_target`. Assume from a non-plugin repo: denied.
3. From a preview host as root: `aws s3 ls` on the bucket is denied; `aws ssm get-parameter` on any parameter is denied; `curl 169.254.169.254` from inside the Mattermost container fails.
4. Label a tactical-fusion PR: the comment reaches `ready`; `https://tactical-fusion-pr<N>.<domain>/api/v4/system/ping` returns 200 with a valid certificate; `/api/v4/plugins/webapp` lists `com.mattermost.plugin-tactical-fusion`; `admin` logs in with the derived password; team `test` exists; the `env` object is gone from S3; the run log shows no PR checkout and no password.
5. Push a commit from an org branch: the comment shows `redeployed` with the new SHA once `pr.yml` finishes, test content persists, a file planted in the plugins volume by the previous commit is gone.
6. Remove the label while an `up` is running, then push a commit: the preview is torn down by the last run to execute.
7. Remove the label: A record deleted before the instance terminates, S3 prefix gone, comment `torn down`. Re-add the label within an hour: a fresh instance is created.
8. Close a labeled PR: torn down. Label a fork PR: it deploys from the `pr.yml` artifact. Push to the fork: torn down, label removed, comment explains why.
9. Post a comment containing the marker from a non-bot account: the bot creates its own comment and never edits the impostor.
10. Upload crafted artifacts on a test PR (two entries, a symlink member, a nested `plugin.json`, an id with a newline, a 300 MB file): `fetch` aborts before any AWS call and the comment says `failed`.
11. `workflow_dispatch` the reaper with `PREVIEW_MAX_AGE_DAYS=0` against a test preview and confirm cleanup, including an orphan A record created by hand.
12. Let a preview time out on purpose (bad image tag): instance tagged `failed`, `env` object deleted, comment `failed`; the next run terminates it and creates a new one.
13. Try to push a branch named `v1` and to move the `v1` tag on `pr-preview` as a non-admin: both refused.

## Open risks

- Anyone with write on a `mattermost-plugin-*` repo can assume the preview role and disrupt previews; the preview zone and account are isolated, so the damage stops there.
- Docker Hub anonymous pull limits from AWS public IPs may slow first boot; add a pull token or a mirror later if it bites.
- An SCP in the dev OU denying internet gateways or public IPs would block the design; check during bootstrap.
- OpenTofu `use_lockfile` needs 1.10 or later; otherwise add a DynamoDB lock table in bootstrap.
- Cost is roughly $2.20 per day per t3.large preview with a public IPv4 address, plus the domain registration; the cap, reaper, and budget alarm bound it.
- Each redeploy waits for `pr.yml`, which runs the full test suite first, so a push takes about 25 minutes to reach the preview. A `workflow_run` trigger would remove the idle runner time if that becomes a problem.
- A maintainer can label a PR whose page has not refreshed since the fork pushed; the comment shows the deployed SHA so the approval is visible after the fact.

## Verification log

- 2026-09-25: first end-to-end run on PR 61 of `mattermost-plugin-tactical-fusion`.
  Two fixes came out of it, both published as `v1`: the Mattermost 11.11
  image ships no `curl` or shell, so the readiness probe is
  `mmctl system status --local`; and `mmctl plugin list --json` emits
  `[{active, inactive}]`, so the parser unwraps the array. After the fixes the
  preview reached `ready` six minutes after the label, served
  `com.mattermost.plugin-tactical-fusion` over HTTPS with a valid certificate,
  accepted an `admin` login with the derived password, and was torn down by
  the label removal and again by the PR close.
