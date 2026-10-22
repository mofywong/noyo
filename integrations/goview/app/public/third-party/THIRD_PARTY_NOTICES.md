# Third-party notices — GoView embedded editor

This file records assets that ship inside the Noyo product but are **not**
covered by the GoView MIT license in `app/LICENSE`. Treat each entry as its own
permission surface; do not read one permissive grant as covering the rest.

## Bundled by the embedded editor build

| Asset | Where it comes from | License / obligation |
| --- | --- | --- |
| GoView source (`app/src`, excluding `src/noyo/` and the files in `patches/PATCHES.md`) | Gitee `dromara/go-view` @ `5ab05b439703ffda6d050e5626969459b3e3c1cf` | MIT — `app/LICENSE` retained in full |
| Runtime dependencies in `app/package-lock.json` (vue, pinia, naive-ui, echarts, lodash, @vitejs/plugin-vue, vite and their transitive trees) | npm registry, versions pinned by the lock file | Each package's own license (MIT/ISC/Apache-2.0 dominate); include the lock file with any redistribution |
| `app/public/favicon.ico` | Upstream GoView demo favicon | Upstream MIT for the code tree does not automatically cover demo artwork; verified as generic GoView branding and **flagged for replacement** before external distribution |

## Not bundled, deliberately

| Asset | Status |
| --- | --- |
| GoViewPro components, template market resources | Commercial upstream product; out of scope for this open-source integration |
| Online map tile services / keys | Not configured in the embedded build; screen map use must go through the Noyo host with deployment-managed keys and an offline GeoJSON fallback |
| Monaco editor workers | Not reachable from the embedded entry; not shipped in production assets |
| Upstream demo images, mock JSON, demo API origins | Removed with the mock/`api` patches; see `patches/PATCHES.md` |

## Ongoing obligation

Any dependency, font, icon, map or plugin added under `integrations/goview/app`
must be added here with its license before release, together with the reason it
is required. A permissive GoView grant never implies permission for third-party
assets.
