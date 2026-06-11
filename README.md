# Contribution Graph Sync

Sync contribution activity from GitLab or Azure DevOps into a target Git repository by creating dated synthetic commits.

## Setup

- `cp .env.template .env`
- Create a target repo into which you want to commit the activity. This path has to be accessible and has to be a git repo already.
- Set `TARGET_SYNC_REPO` to the absolute path of that repo.
- Configure GitLab, Azure DevOps, or both below.

## GitLab

- [Create a GitLab token](https://gitlab.com/-/user_settings/personal_access_tokens) with the `api` scope.
- Set `GL_API_TOKEN`.

Existing `.env` files that use `GL_TARGET_SYNC_REPO` still work, but `TARGET_SYNC_REPO` is preferred.

## Azure DevOps

This tool uses [Microsoft Entra ID (azidentity)](https://learn.microsoft.com/en-us/azure/developer/go/azure-sdk-authentication) to securely authenticate with Azure DevOps. Use `az login` (or `az login --allow-no-subscriptions`) to authenticate locally or configure managed identities/service principals.

- Set `AZDO_ORGANIZATION` to the organization slug from `https://dev.azure.com/<organization>`.
- Set `AZDO_AUTHOR` to the exact name or email you author Git commits with (e.g. `Tomáš Diblík`). Azure DevOps requires this to accurately filter your commits.

Azure DevOps mode syncs authored commits and opened pull requests.

## Run

- `go run ./src`
