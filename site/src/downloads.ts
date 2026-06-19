// Resolve the latest GitHub release assets for the download buttons.
// Falls back gracefully to the releases page when anything goes wrong.

const RELEASES_PAGE = 'https://github.com/kindlyops/hyperdeck-adapter/releases';
const LATEST_API =
  'https://api.github.com/repos/kindlyops/hyperdeck-adapter/releases/latest';

interface ReleaseAsset {
  name: string;
  browser_download_url: string;
}

interface Release {
  tag_name?: string;
  assets?: ReleaseAsset[];
}

function setButton(
  buttonId: string,
  metaId: string,
  asset: ReleaseAsset | undefined,
  tag: string | undefined,
): void {
  const button = document.getElementById(buttonId) as HTMLAnchorElement | null;
  const meta = document.getElementById(metaId);
  if (!button) return;
  if (asset) {
    button.href = asset.browser_download_url;
    if (meta) {
      meta.textContent = tag ? `${asset.name} · ${tag}` : asset.name;
    }
  } else {
    button.href = RELEASES_PAGE;
  }
}

export function wireDownloadButtons(): void {
  fetch(LATEST_API, { headers: { Accept: 'application/vnd.github+json' } })
    .then((res) => {
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      return res.json() as Promise<Release>;
    })
    .then((release) => {
      const assets = release.assets ?? [];
      const mac = assets.find((a) => a.name.toLowerCase().endsWith('.dmg'));
      const win = assets.find((a) => {
        const n = a.name.toLowerCase();
        return n.endsWith('.exe') || n.endsWith('.msi');
      });
      // Prefer the .deb; fall back to the tarball.
      const linux =
        assets.find((a) => a.name.toLowerCase().endsWith('.deb')) ??
        assets.find((a) => a.name.toLowerCase().endsWith('.tar.gz'));
      setButton('dl-mac', 'dl-mac-meta', mac, release.tag_name);
      setButton('dl-win', 'dl-win-meta', win, release.tag_name);
      setButton('dl-linux', 'dl-linux-meta', linux, release.tag_name);
    })
    .catch(() => {
      // Buttons already point at the releases page in the HTML; nothing to do.
    });
}
