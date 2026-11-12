"""MkDocs hook: stub pages for the compliance engine docs when they are absent.

The "For engine authors" pages (docs/site/compliance/engines/*.md) are owned by
gorge-compliance. The docs workflow copies them in from that repo's
docs/engines/ before `mkdocs build`. A local build has no such copy, and the
nav in mkdocs.yml still names every page, so under --strict each missing one
would fail. This hook adds a stub for each page that is not on disk, pointing
at the compliance repo. When the pages are copied in, nothing is stubbed.
"""

from mkdocs.structure.files import File

REPO = "https://github.com/adams-shaun/gorge-compliance"
PAGES = (
    "index.md",
    "quickstart.md",
    "protocol.md",
    "howto.md",
    "results.md",
    "engine-notes.md",
    "faq.md",
)


def on_files(files, config):
    for name in PAGES:
        src = "compliance/engines/" + name
        if files.get_file_from_path(src) is not None:
            continue
        body = (
            "# Bring your engine\n\n"
            "This page is published from the gorge-compliance repository and is "
            "not part of this local build. Read it at "
            f"[docs/engines/{name}]({REPO}/blob/main/docs/engines/{name}), or "
            f"browse [gorge-compliance]({REPO}).\n"
        )
        files.append(File.generated(config, src, content=body))
    return files
