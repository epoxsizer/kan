# Security policy

## Supported versions

Security fixes are applied to the latest published version. Before version 1.0,
users should upgrade to the newest release rather than expect patches for older
minor versions.

## Reporting a vulnerability

Use GitHub Security Advisories when they are enabled for the repository. If they
are not available yet, open a public issue with only a high-level description and
request a private contact path from the maintainers. Do not include private board
or card data in a public issue. Include the affected version, operating system,
reproduction steps, and impact in the private report. Acknowledgement should
arrive within seven days.

kan is local-first and performs no application network requests, but imported JSON,
SQLite databases, terminal rendering, lock files, backups, and filesystem permissions
remain part of its security surface.
