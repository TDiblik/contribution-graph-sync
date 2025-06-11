# Contribution Graph Sync

Project used to sync activity graph beween GitHub and GitLab. (only works in GL->GH direction) <br />

Setup:

- [Create GitLab token](https://gitlab.com/-/user_settings/personal_access_tokens) with the `api` scope.
- `cp .env.template .env`
- Create a target repo into which you wanna commit the activity. This path has to be accessible AND has to be a git repo already!
- Update `.env` with the two parameters above.
  - The `GL_TARGET_SYNC_REPO` has to be an absolute path.

Run:

- `go run ./src`
