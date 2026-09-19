# Recording API samples

One script per sample in `gcp/samples/data`. Each script shows the `curl` request, and saves the response to `out/<sample name>.json`.

`out/` is git-ignored, because the responses hold real identifiers. Replace them with placeholders before a file goes into `gcp/samples/data`.

## Usage

Log in once with each account you want to call as, then run a script and answer the questions:

```sh
gcloud auth login            # once per account, see: gcloud auth list
./projects.get.error.non_existent.sh
```

- **Who to call as.** The first run shows a menu of the logged-in accounts (`gcloud auth list`), then of the projects that account can see (`gcloud projects list`). The chosen pair is remembered in `out/.recent`, so the next run offers it as number 1. The last 10 pairs are kept.
- **Quota project.** The project the API calls are counted against. It is sent as `x-goog-user-project`. If it is not in the list, choose "a project ID that is not listed" and type it.
- **Other values.** A script that needs a project or an operation asks for it, so it can be pasted: a project number or ID for `PROJECT`, or `operations/dp.…` for `OPERATION`. An answer that does not look right stops the script.
- **Without a menu.** Set `ACCOUNT`, `QUOTA_PROJECT` and the script's own variable (`PROJECT`, `EXISTING_PROJECT_ID`, `OPERATION`) and nothing is asked, so a script also runs where there is no terminal:

  ```sh
  ACCOUNT=me@example.com QUOTA_PROJECT=my-quota-project PROJECT=123456789012 \
    ./projects.get.success.with_parent.sh
  ```

The script says who it calls as, so the response can be tied to that account.

## After recording

Replace the real identifiers, and write the sample into `gcp/samples/data`:

```sh
go run ./cmd/anonymize gcp/samples/scripts/out/projects.get.error.non_existent.json
```

Look at the result with `git diff`, and add the sample to the README of `gcp/samples/data`. See "Placeholders" there for what is replaced.

## Scripts

| Script | Extra variable | Safe? |
|---|---|---|
| `projects.get.error.invalid_argument.sh` | | read only |
| `projects.get.error.non_existent.sh` | | read only |
| `projects.get.error.permission_denied.sh` | `PROJECT` (owned by someone else) | read only |
| `projects.get.success.with_parent.sh` | `PROJECT` (ACTIVE, has a parent) | read only |
| `projects.get.success.delete_requested.sh` | `PROJECT` (deleted) | read only |
| `operations.get.success.delete_done.sh` | `OPERATION` | read only |
| `projects.create.error.invalid_argument.sh` | | rejected, creates nothing |
| `projects.create.error.already_exists.sh` | `EXISTING_PROJECT_ID` | rejected, creates nothing |
| `projects.delete.error.non_existent.sh` | | random ID, deletes nothing |
| `projects.delete.error.already_deleted.sh` | `PROJECT` (deleted) | refuses unless the project is DELETE_REQUESTED |

## Not scripted

These would create or delete real projects, so they are left out for now:

- `projects.create.error.permission_denied`: it needs an account that may not create projects. A valid request from an account that may would create a project.
- `operations.get.success.running`, `operations.get.error` and `projects.delete.success`: they need a project to be created or deleted. Take them from a lifecycle run.
- `operations.get.success.caller_*` and `projects.get.success.no_parent`: they create a project as another account.
- `projects.delete.error.permission_denied`: it deletes a project owned by someone else. If the account is allowed, it is deleted.
