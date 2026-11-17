import { useEffect, useMemo, useState } from "react";

interface CreatorAsset {
  path: string;
  title: string;
  category: string;
  collection: string;
  description: string;
  useCase: string;
  width: number;
  height: number;
  preview: string;
  tags: string[];
}

interface CreatorCatalog {
  libraryRoot: string;
  assets: CreatorAsset[];
}

const apiBase = import.meta.env.VITE_API_BASE_URL ?? "";
const pathURL = (path: string) => path.split("/").map(encodeURIComponent).join("/");

function obsFileURL(root: string, assetPath: string, title: string, subtitle: string) {
  const base = "file://" + pathURL(root.replace(/\/+$/, "") + "/" + assetPath);
  const params = new URLSearchParams();
  if (title.trim()) params.set("title", title.trim());
  if (subtitle.trim()) params.set("subtitle", subtitle.trim());
  return params.size ? base + "?" + params.toString() : base;
}

export function CreatorAssets() {
  const [catalog, setCatalog] = useState<CreatorCatalog | null>(null);
  const [error, setError] = useState("");
  const [search, setSearch] = useState("");
  const [category, setCategory] = useState("all");
  const [selectedPath, setSelectedPath] = useState("");
  const [title, setTitle] = useState("");
  const [subtitle, setSubtitle] = useState("");
  const [copyStatus, setCopyStatus] = useState("");

  useEffect(() => {
    let active = true;
    void fetch(apiBase + "/api/v1/creator-assets/catalog")
      .then(async (response) => {
        if (!response.ok) {
          const detail = (await response.json().catch(() => ({}))) as { error?: string };
          throw new Error(detail.error ?? "Could not load creator assets.");
        }
        return response.json() as Promise<CreatorCatalog>;
      })
      .then((payload) => { if (active) setCatalog(payload); })
      .catch((reason: unknown) => {
        if (active) setError(reason instanceof Error ? reason.message : "Asset library unavailable.");
      });
    return () => { active = false; };
  }, []);

  const categories = useMemo(() => {
    return [...new Set(catalog?.assets.map((asset) => asset.category) ?? [])].sort();
  }, [catalog]);

  const matching = useMemo(() => {
    const term = search.trim().toLowerCase();
    return (catalog?.assets ?? []).filter((asset) => {
      const matchesCategory = category === "all" || asset.category === category;
      const text = [asset.title, asset.collection, asset.category, asset.description, asset.useCase, ...(asset.tags ?? [])].join(" ").toLowerCase();
      return matchesCategory && (!term || text.includes(term));
    });
  }, [catalog, search, category]);

  const selected = catalog?.assets.find((asset) => asset.path === selectedPath);
  const obsURL = selected && catalog
    ? obsFileURL(catalog.libraryRoot, selected.path, title, subtitle)
    : "";

  async function copySourceURL() {
    if (!obsURL) return;
    try {
      if (!navigator.clipboard?.writeText) throw new Error("Clipboard is unavailable");
      await navigator.clipboard.writeText(obsURL);
      setCopyStatus("Copied. In OBS, add a Browser Source with this URL and use the listed dimensions.");
    } catch {
      setCopyStatus("Copy the URL from the field below. Clipboard access may require HTTPS or localhost.");
    }
  }

  return (
    <section className="panel panel--wide" aria-labelledby="creator-assets-heading">
      <div className="panel__header">
        <div>
          <p className="eyebrow">Asset Studio</p>
          <h2 id="creator-assets-heading">Creator Asset Library</h2>
          <p className="creator-assets__hint">Browse locally installed OBS overlays, customize copy, and prepare Browser Source URLs.</p>
        </div>
        <span className="status-badge status-badge--healthy">{catalog ? catalog.assets.length + " assets" : "Optional"}</span>
      </div>

      {error ? (
        <div className="creator-assets__empty" role="status">
          <strong>Asset library not connected</strong>
          <p>{error}</p>
          <p>Install OBS-Scene-Assets under /data/creator-assets and restart the BitRiver control API. See docs/CREATOR_ASSETS.md.</p>
        </div>
      ) : !catalog ? (
        <p className="creator-assets__hint" role="status">Loading catalog...</p>
      ) : (
        <>
          <div className="creator-assets__filters">
            <label>
              Search assets
              <input type="search" value={search} onChange={(event) => setSearch(event.target.value)} placeholder="Starting soon, podcast, gaming..." />
            </label>
            <label>
              Category
              <select value={category} onChange={(event) => setCategory(event.target.value)}>
                <option value="all">All categories</option>
                {categories.map((entry) => <option value={entry} key={entry}>{entry}</option>)}
              </select>
            </label>
          </div>
          <p className="creator-assets__hint">{matching.length} matching assets. Showing the first 18; refine your search to find more.</p>
          <div className="creator-assets__grid">
            {matching.slice(0, 18).map((asset) => (
              <button
                key={asset.path}
                type="button"
                className={"creator-assets__card" + (selectedPath === asset.path ? " creator-assets__card--selected" : "")}
                aria-pressed={selectedPath === asset.path}
                onClick={() => { setSelectedPath(asset.path); setTitle(""); setSubtitle(""); setCopyStatus(""); }}
              >
                <img src={apiBase + "/api/v1/creator-assets/preview/" + pathURL(asset.preview)} alt="" loading="lazy" />
                <strong>{asset.title}</strong>
                <small>{asset.collection} · {asset.width}×{asset.height}</small>
              </button>
            ))}
          </div>
          {matching.length === 0 && <p className="creator-assets__hint">No matching assets. Try another search.</p>}
          {selected && (
            <div className="creator-assets__editor">
              <div>
                <strong>{selected.title}</strong>
                <p>{selected.useCase || selected.description}</p>
                <p>Use a {selected.width} × {selected.height} OBS Browser Source. Copying a URL does not install a source automatically.</p>
              </div>
              <div className="creator-assets__filters">
                <label>
                  Custom title (optional)
                  <input value={title} onChange={(event) => setTitle(event.target.value)} placeholder="Your show title" />
                </label>
                <label>
                  Subtitle (optional)
                  <input value={subtitle} onChange={(event) => setSubtitle(event.target.value)} placeholder="Episode or segment" />
                </label>
              </div>
              <div className="creator-assets__copy">
                <button className="button button--secondary" type="button" onClick={() => void copySourceURL()}>Copy OBS Browser Source URL</button>
                <input aria-label="OBS Browser Source URL" readOnly value={obsURL} onFocus={(event) => event.target.select()} />
              </div>
              {copyStatus && <p className="creator-assets__hint" role="status">{copyStatus}</p>}
            </div>
          )}
        </>
      )}
    </section>
  );
}
