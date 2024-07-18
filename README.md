# [gitr-backup](https://github.com/alixinne/gitr-backup)

gitr-backup is a tool to mirror personal repositories from one Git hosting
service to another. This is mainly used for backing up repositories on
platforms that do not natively support pull mirrors (Gitea, GitHub, GitLab CE).

This is a successor (and rewrite) of
[gem-repositories](https://github.com/alixinne/gem-repositories). This
rewrite is not complete and does not cover the entire feature set of
gem-repositories:

| Platform (usage) |    gitr-backup     |  gem-repositories  |
| :--------------: | :----------------: | :----------------: |
| GitHub (source)  | :heavy_check_mark: | :heavy_check_mark: |
| GitHub (backup)  |        :x:         | :heavy_check_mark: |
| GitLab (source)  |        :x:         | :heavy_check_mark: |
| GitLab (backup)  |        :x:         | :heavy_check_mark: |
|  Gitea (source)  | :heavy_check_mark: | :heavy_check_mark: |
|  Gitea (backup)  | :heavy_check_mark: | :heavy_check_mark: |

## Getting started

### Running from source

To run this project from source:

```bash
git clone --recurse-submodules https://github.com/alixinne/gitr-backup.git
cd gitr-backup
make -C git2go install-static
go run -tags static .
```

### Running with Docker

This project is distributed as a Docker image built using GitHub Actions.

```bash
docker run --rm -it -v $PWD/config.yaml:/config.yaml:ro ghcr.io/alixinne/gitr-backup:0.2.0
```

### Configuration file

This tool relies on a configuration file (`config.yaml`) being present in the
current directory. This configuration files describes what are the source and
destination Git hosts to mirror. Here is a sample configuration file:

```yaml
hosts:
  - type: github
    token: $GITHUB_TOKEN # Fetched from environment variable
    use_as: source
  - type: gitea
    base: https://gitea.example.com
    token: $GITEA_API_TOKEN # Fetched from environment variable
    use_as: backup
```

In this configuration file, repositories owned by the user whose token is
contained in the `GITHUB_TOKEN` environment variable will be mirrored as
repositories owned by the user whose token is `GITEA_API_TOKEN` on the Gitea
host gitea.example.com.

If you specify multiple source hosts, they will all be replicated to the backup
hosts.

If you specify multiple backup hosts, they will all get a mirror of all the
source repositories.

## Author

Alixinne <alixinne@pm.me>
