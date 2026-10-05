# Homebox custom inventory fork

## Baseline

- Upstream: https://github.com/sysadminsmedia/homebox (Git remote `upstream`).
- Stable release checked on 2026-10-05: **v0.26.2**.
- Baseline commit: **e01dd737238a3fa7e1a6454b37de6c6fc88c86e4**.
- Fork: https://github.com/FelberPeter/homebox-custom.
- Development branch: `custom/mobile-inventory`.

The upstream contribution instructions were reviewed. Existing entities,
location trees, authentication, tenant middleware, tags, fields and attachment
storage are reused. Financial and insurance database columns are retained;
the normal UI no longer exposes those fields or maintenance navigation.
Projects remain tags. There is no project or maintenance management extension.

## Location operations

`POST /api/v1/locations/{id}/operations` uses the existing authenticated,
group-scoped middleware. Set `preview: true` to review names, selected nodes,
excluded nodes, counts and conflicts. Execute the same request with
`preview: false`. Each logical operation needs a fresh UUID `requestId`.
Reusing a completed ID with changed input or a different source is rejected.
Retries with the original input return the original result, including after
restarts. A persistent, group-scoped operation ledger is written in the same
transaction as the inventory changes. Receipts survive deletion of a copy.

Number mode replaces exactly one `*`. Grid mode replaces exactly one `{row}`
and one `{col}`. Rows are uppercase A-Z, numbers are nonnegative integers,
counts are positive, number padding is 0-8 digits. The total generation or
copy limit is 200 records. No executable templates are supported.

Copy depth is location depth: 0=root only, 1=direct locations, 2=their children,
-1=all. Item descendants within selected locations are copied recursively
only when items are enabled. Their parent-child relationships and quantities
are retained. A location beneath an excluded location or item is excluded,
with all its descendants. Excluded records are listed in the preview.
Copied records have new UUIDs and new group asset IDs. Existing tags are
reused; custom fields and notes are copied. Child location names remain literal.
Purchase/warranty and serial details default to excluded. Financial,
insurance, sold and maintenance details are not copied by these operations.

Photos default on; other attachments default off. Each copied file, including
thumbnails, has its own new storage key. External URL attachments retain their
URLs rather than downloading remote content. Deleting one side cannot remove
the other side's file. File failures abort and roll back every database change;
normal failure paths clean up staged files. Process termination between file
staging and commit can leave unreferenced `copy-*` blobs, but cannot commit a
partial inventory tree. Retry the unchanged operation safely. This deployment
runs one application replica; concurrent application replicas are not supported
for these custom operations.

Moves change only the root name and parent association. Descendant IDs,
asset IDs, contents, attachments and QR targets remain stable. Self/descendant
moves are rejected, including through the regular edit APIs. Same-name sibling
locations require explicit confirmation in the new dialogs and normal UI forms.
Nothing is replaced, merged or automatically renamed.

## Build and deployment

The full source build is in `Dockerfile.custom`. Node, Go and Alpine images
are pinned by digest; frontend and backend dependencies use upstream lockfiles.
The image embeds its Git commit in `/api/v1/status` and OCI labels. Image
compression uses the upstream pure-Go/WASM decoders (`nodynamic`).

The separate Compose project is `homebox-custom` under
`/home/server/homebox-custom-src/deploy` on the server. The installation uses
its own `data/` directory; no existing database is imported or modified.

```sh
cd /home/server/homebox-custom-src/deploy
# Initial setup only, keep .env private and out of Git:
umask 077
printf 'CUSTOM_COMMIT=%s\nHOMEBOX_PORT=7746\nHOMEBOX_BIND_IP=0.0.0.0\nHBOX_AUTH_API_KEY_PEPPER=%s\n' \
  "$(git -C .. rev-parse HEAD)" "$(openssl rand -hex 32)" > .env

docker compose build
docker compose up -d
docker compose ps
# Stop / start without removing data:
docker compose stop
docker compose start
```

Use http://homeserver:7746 (LAN IP: 192.168.254.100). No public DNS, router
forwarding or public proxy endpoint is configured by this deployment. TZ is
Europe/Berlin. Restart policy is unless-stopped; health checks request the
status endpoint. The browser's file/photo picker is retained on HTTP, including
Android capture selection when the browser offers it. Camera APIs using
getUserMedia, the AR scanner and service worker features require a browser-
trusted HTTPS origin; direct LAN HTTP does not provide that. Existing proxy
configuration is unchanged. HTTPS can be added using the user's trusted local
certificate/proxy infrastructure separately.

### Update

Back up first. Fetch and review changes, then check out the chosen custom
commit. Preserve .env and data. Update CUSTOM_COMMIT in .env to the chosen
commit, then `docker compose build && docker compose up -d`. Review status
and the embedded build commit. Updating to upstream requires merging the
chosen release into the custom branch and rerunning the custom tests; using
an upstream image directly would remove the custom UI and operations.

### Backup and restore

Run `sh backup.sh` from deploy/. It briefly stops this project, archives data,
.env and compose.yaml together, then restarts. Backups contain private data
and secrets; keep them private. Back up source commit and Dockerfile as well.

For restore, stop this project, move its current data directory to a dated
recovery directory, and extract the chosen archive into an empty deployment
directory. Do not overwrite an active database. Restore the matching source
commit and image tag, then start Compose. Keep the replaced directory until
restored data and uploads have been checked. No other Homebox instance should
be targeted by these commands.

## Verification

The complete Go suite and targeted repository and handler tests passed. They cover generated names,
limits/pattern validation, repeated generation, conflict consent, copy depths,
item inclusion, nested items, quantities, fresh/stable IDs, tenant isolation,
individual data switches, independent files, operation replay and rollback.
The frontend production build, changed-file ESLint checks and 19 focused Vitest tests passed. The unchanged upstream release has
198 TypeScript errors in this Windows dependency environment; this is tracked
separately from production build and custom-feature tests. Final deployment,
browser, backup restoration and persistence results are recorded in `ACCEPTANCE.md`.
A short German operating guide is provided in `BETRIEB.md`.
