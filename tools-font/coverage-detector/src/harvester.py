"""
Text harvester — collect renderable text runs from EPUB XHTML documents.

Each ``TextRun`` captures a contiguous piece of rendered text together with
its location in the document, surrounding context, and CSS ancestry — all
needed by downstream font coverage analysis.

Sources collected
    Body text, headings, figcaption, ``img`` / ``image`` / ``area`` [alt]
    attributes, ``nav`` / ``toc`` content, footnote / duokan-note elements,
    cover page content.

Sources excluded
    ``<script>``, ``<style>`` content (CSS generated content cannot be
    detected from XHTML alone); purely decorative whitespace.
"""

from lxml import etree


__all__ = ["harvest_runs"]

# ---------------------------------------------------------------------------
# Tag classification
# ---------------------------------------------------------------------------

# Elements whose direct text content we collect.
COLLECT_TAGS = frozenset((
    "p", "div", "span",
    "h1", "h2", "h3", "h4", "h5", "h6",
    "li", "a", "blockquote", "pre", "code",
    "td", "th", "dt", "dd",
    "figcaption", "caption", "q", "cite",
    "em", "strong", "b", "i",
    "u", "s", "small", "sub", "sup",
    "abbr", "address", "label", "legend",
    "summary", "nav",
))

# Elements whose ``alt`` attribute we treat as text runs.
ALT_TAGS = frozenset((
    "img", "image", "area",
))

# Elements skipped entirely (their text is not rendered as page content).
SKIP_TAGS = frozenset((
    "script", "style",
    "svg", "math",
    "iframe", "object", "canvas",
    "noscript", "link", "meta",
))

# Heading elements (collected as body text, but tagged differently).
HEADING_TAGS = frozenset((
    "h1", "h2", "h3", "h4", "h5", "h6",
))

# EPUB note / footnote class / role / epub:type indicators.
_NOTE_INDICATORS = frozenset((
    "footnote", "footnotes", "footnoteref",
    "endnote", "endnotes",
    "noteref",
    "duokan-footnote", "duokan-note", "duokanannotation",
))


# ---------------------------------------------------------------------------
# DOM helpers
# ---------------------------------------------------------------------------

def _tag_name(el):
    """
    Return the local tag name of *el*, stripping any XML namespace prefix.

    Example
    -------
    ``{http://www.w3.org/1999/xhtml}p``  →  ``"p"``
    """
    tag = el.tag
    if not isinstance(tag, str):
        # Comment / processing-instruction node — not an element.
        return ""
    if "}" in tag:
        return tag.rsplit("}", 1)[1].lower()
    return tag.lower()


def _get_epub_type(el):
    """
    Return the ``epub:type`` attribute value, trying several namespace forms
    that real-world EPUB producers use.
    """
    raw = el.get("epub:type")
    if raw:
        return raw.lower()
    # Parsed as non-prefixed attribute in some XML serialisations.
    raw = el.get("{http://www.idpf.org/2007/ops}type")
    if raw:
        return raw.lower()
    return ""


def _get_inline_style(el):
    """Return the *style* attribute value (stripped) or ``None``."""
    value = el.get("style")
    return value.strip() if value else None


def _is_note_element(el):
    """
    Return ``True`` if *el* is likely a footnote, endnote, or duokan-style
    annotation (detected via class, ``epub:type``, or ``role``).
    """
    cls = (el.get("class") or "").lower()
    etype = _get_epub_type(el)
    role = (el.get("role") or "").lower()

    for indicator in _NOTE_INDICATORS:
        if indicator in cls or indicator in etype or indicator in role:
            return True
    return False


def _build_ancestor_chain(el):
    """
    Build an ancestor chain from document root to *el*.

    Each entry is a dict with keys: tag, class (str or None), id (str or None).
    e.g. [{"tag": "html"}, {"tag": "body"}, {"tag": "div", "class": "content", "id": "intro"}]

    An empty chain is returned for non-element nodes.
    """
    chain = []
    current = el
    while current is not None:
        tag = _tag_name(current)
        if not tag:
            break
        entry = {"tag": tag}
        cls = (current.get("class") or "").strip()
        if cls:
            entry["class"] = cls
        eid = (current.get("id") or "").strip()
        if eid:
            entry["id"] = eid
        chain.append(entry)
        current = current.getparent()
    chain.reverse()
    return chain


def _build_node_path(el):
    """
    Build a simplified XPath-like path to *el*.

    Example
    -------
    ``/html/body/div[2]/p[3]/span[1]``
    """
    parts = []
    current = el
    while current is not None:
        tag = _tag_name(current)
        if not tag:
            break

        parent = current.getparent()
        if parent is not None:
            # Count preceding siblings with the same tag name.
            # iterchildren(tag=...) filters by tag string, skipping comments/pi.
            siblings = list(parent.iterchildren(tag=current.tag))
            try:
                index = siblings.index(current) + 1
            except ValueError:
                index = 1
        else:
            index = 1

        parts.append(f"{tag}[{index}]")
        current = parent

    parts.reverse()
    return "/" + "/".join(parts)


# ---------------------------------------------------------------------------
# Tree walker
# ---------------------------------------------------------------------------

def _walk(element, file_href, text_parts, runs):
    """
    Depth-first walk of *element*, accumulating into *text_parts* (all text
    nodes in document order for full-text reconstruction) and *runs* (the
    TextRun dicts we will return).

    Parameters
    ----------
    element : lxml.Element
        Current node to process (called recursively on children).
    file_href : str
        EPUB-internal path of the XHTML file being processed.
    text_parts : list[str]
        Grows with every text node (including whitespace, tails).
    runs : list[dict]
        Grows with each discoverd TextRun.
    """
    tag = _tag_name(element)
    if not tag or tag in SKIP_TAGS:
        return

    # ------------------------------------------------------------------
    # 1. Element's own direct text (text before first child element).
    # ------------------------------------------------------------------
    raw_text = element.text
    if raw_text:
        text_parts.append(raw_text)

        stripped = raw_text.strip()
        if stripped and not stripped.isspace() and tag in COLLECT_TAGS:
            _record_run(element, file_href, stripped,
                        raw_text, len(text_parts) - 1,
                        text_parts, runs,
                        element_tag=tag)

    # ------------------------------------------------------------------
    # 2. Recurse into child elements.
    # ------------------------------------------------------------------
    for child in element.iterchildren():
        _walk(child, file_href, text_parts, runs)

        # -- Tail text of child (text after child, before next sibling) --
        tail = child.tail
        if tail:
            text_parts.append(tail)

            stripped = tail.strip()
            parent = child.getparent()
            if (stripped and not stripped.isspace()
                    and parent is not None):
                parent_tag = _tag_name(parent)
                if parent_tag in COLLECT_TAGS:
                    _record_run(parent, file_href, stripped,
                                tail, len(text_parts) - 1,
                                text_parts, runs,
                                element_tag=parent_tag)

    # ------------------------------------------------------------------
    # 3. Alt-text on image / area elements.
    # ------------------------------------------------------------------
    if tag in ALT_TAGS:
        alt = element.get("alt", "").strip()
        if alt:
            # Alt text is not a regular text node — we add it separately
            # to the full-text accumulator so context calculations work.
            text_parts.append(alt)
            _record_run(element, file_href, alt,
                        alt, len(text_parts) - 1,
                        text_parts, runs,
                        element_tag=f"{tag}[alt]")


def _record_run(el, file_href, stripped, raw_text, text_index,
                 text_parts, runs, *, element_tag):
    """
    Build a TextRun dict and append it to *runs*.

    *stripped* is the clean text that goes into the ``text`` field.
    *raw_text* and *text_index* are bookkeeping for offset/context later.
    """
    # Detect semantic subtype.
    tag = _tag_name(el)
    etype = element_tag  # default
    if tag in HEADING_TAGS:
        etype = "heading"
    elif tag == "figcaption":
        etype = "figcaption"
    elif tag == "nav":
        etype = "nav"
    elif _is_note_element(el):
        etype = "note"

    run = {
        "text": stripped,
        "file": file_href,
        "element_tag": etype,
        "ancestor_chain": _build_ancestor_chain(el),
        "inline_style": _get_inline_style(el),
        "node_path": _build_node_path(el),
        "offset": 0,      # computed in phase 2
        "context": "",     # computed in phase 2
    }
    # Internal bookkeeping — cleaned up after phase 2.
    run["_raw_text"] = raw_text
    run["_text_index"] = text_index
    runs.append(run)


# ---------------------------------------------------------------------------
# Context / offset resolution
# ---------------------------------------------------------------------------

def _resolve_offsets_and_contexts(text_parts, runs, window=20):
    """
    Second pass: for each run, compute its character *offset* in the full
    document text and a *context* window of up to *window* characters on
    either side.

    Modifies *runs* in place and removes internal bookkeeping keys.
    """
    # Build the concatenated full text and record the start position of
    # each *text_parts* entry within it.
    full_text = ""
    part_starts = []
    for part in text_parts:
        part_starts.append(len(full_text))
        full_text += part

    for run in runs:
        start = run.get("_text_index")
        raw = run.get("_raw_text", "")

        if start is not None and 0 <= start < len(part_starts):
            pos = part_starts[start]
            run["offset"] = pos

            # Window: up to <window> chars before, up to <window> after.
            ctx_start = max(0, pos - window)
            ctx_end = min(len(full_text), pos + window + len(raw))
            run["context"] = full_text[ctx_start:ctx_end]

        # Remove internal keys.
        run.pop("_raw_text", None)
        run.pop("_text_index", None)


# ---------------------------------------------------------------------------
# Public API
# ---------------------------------------------------------------------------

def harvest_runs(zf, xhtml_docs):
    """
    Extract all text runs from the XHTML documents in an EPUB.

    Parameters
    ----------
    zf : zipfile.ZipFile
        An open ``ZipFile`` for the EPUB.  The caller is responsible for
        closing it (or the EPUB is already exhausted when *harvest_runs*
        returns).
    xhtml_docs : list[dict]
        List of document descriptors produced by ``reader.read_epub()``.
        Each dict must have at least an ``href`` key; entries with a
        ``resolved_path`` key are preferred for ZIP reading (see
        ``reader.read_epub`` which adds both).

    Returns
    -------
    list[dict]
        TextRun dicts with the following keys::

            text           : str    — trimmed text content
            file           : str    — original OPF href of the source XHTML
            element_tag    : str    — ``"heading"``, ``"note"``,
                                      ``"figcaption"``, ``"nav"``,
                                      ``"img[alt]"``, or ``element_tag``
            ancestor_chain : list   — CSS-selector-like ancestry strings
            inline_style   : str | None
            node_path      : str    — simplified XPath to the element
            offset         : int    — character offset in document full-text
            context        : str    — ±20 chars window around the run

    Notes
    -----
    The full-text used for *offset* and *context* is built from **all** text
    nodes in document order (including whitespace), so the context window can
    span across element boundaries within the same file.
    """
    all_runs = []

    for doc in xhtml_docs:
        href = doc["href"]
        # Prefer the resolved path if the reader provided one.
        zip_path = doc.get("resolved_path", href)

        try:
            content = zf.read(zip_path)
        except KeyError:
            # Try the original href as a fallback.
            try:
                content = zf.read(href)
            except KeyError:
                # Cannot read this file — skip silently.
                continue

        # Parse as HTML first (lenient, handles quirks of real-world EPUBs).
        # If that fails, try strict XML parsing.
        parser = etree.HTMLParser(recover=True)
        try:
            tree = etree.fromstring(content, parser)
        except (etree.XMLSyntaxError, etree.ParserError):
            try:
                tree = etree.fromstring(content)
            except (etree.XMLSyntaxError, etree.ParserError):
                # Unparseable — skip.
                continue

        # Phase 1: single-pass collection.
        text_parts = []
        runs = []
        _walk(tree, href, text_parts, runs)

        # Phase 2: offset and context from accumulated text.
        _resolve_offsets_and_contexts(text_parts, runs)

        all_runs.extend(runs)

    return all_runs
