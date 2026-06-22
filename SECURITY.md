# Security policy

## Supported versions

Security fixes are applied to the latest published version. Before version 1.0,
users should upgrade to the newest release rather than expect patches for older
minor versions.

## Reporting a vulnerability

Create a [confidential GitLab issue](https://gitlab.digital-spirit.ru/solutions/common/kan/-/issues/new?issue%5Bconfidential%5D=true).
Confirm that the issue is marked confidential before adding details. Do not include
private board or card data in a public issue. Include the affected version, operating
system, reproduction steps, and impact. Acknowledgement should arrive within seven days.

kan is local-first and performs no application network requests, but imported JSON,
SQLite databases, terminal rendering, lock files, backups, and filesystem permissions
remain part of its security surface.
