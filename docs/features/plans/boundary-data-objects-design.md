# Design: Boundary Data Objects in the Hexagonal-DDD Example

_Status: Draft (design approved 2026-07-27) — spec for review before an implementation plan._

_Part of the DDD / Hexagonal initiative ([#570](https://github.com/docToolchain/Bausteinsicht/issues/570)). Builds on the `examples/hexagonal-ddd` restructure shipped in PR #571 (nested rings)._

## Goal

Show, in the `hexagonal-ddd` example, **how data objects are transformed at the boundaries** of a hexagonal architecture — the ports speak domain objects toward the core, and each adapter maps them to its own outer representation (JPA entity, REST DTO, UI view-model). Make the *pattern* visible and teachable **without mirroring the code's type graph** (the "seam" from the #570 plan — no class zoo, no double maintenance).

Chosen approach (from the design discussion): **B — lightweight model elements in a dedicated `data-mapping` view.** The domain side already exists in the model (`order` aggregate, `line`, `total`); only the adapter side is new. The concentric-ring views stay untouched.

## Scope

**In scope**
- One new element `kind`: `boundaryObject` — the adapter-side data shape (DTO / JPA entity / view-model).
- One new relationship `kind`: `maps` (dashed) — from a `boundaryObject` to its domain counterpart.
- A light association edge tying each `boundaryObject` to its owning adapter (proximity + ownership).
- Exactly **two exemplar** boundary objects (both boundary directions), not an exhaustive set.
- A new `data-mapping` view (layout `layered`).
- Template styling for `boundaryObject` and the `maps` connector.
- A README section explaining the pattern and the seam.

**Out of scope**
- Modelling every DTO/field/class (explicitly — that is the anti-pattern the seam guards against).
- New domain-side kinds (domain objects are reused).
- Any change to the `ports-and-adapters` / `domain-model` (nested-ring) views.
- An ADR (deferred — see Follow-ups).

## Model additions

### Element kind
- `boundaryObject` — "a data object living at an adapter boundary (DTO / persistence entity / view-model)." Not a container.

### Relationship kind
- `maps` — `{ "notation": "maps", "dashed": true }`. Direction: `boundaryObject → domainObject` ("maps to/from the domain object"). Dashed to read as a mapping, not a runtime dependency.

### Light adapter association
- Reuse the existing `uses` (or a dedicated light kind if `uses` reads wrong) as `adapter → boundaryObject` ("produces/owns"). This gives the graphical proximity to the port/adapter the user asked for and encodes ownership. Final direction/kind confirmed during implementation so it stays acyclic (see Constraints).

### Exemplars (top-level elements, siblings of the context — NOT children of ports)
- `placeOrderRequest` — REST/driving/inbound boundary object; `maps → ordering.adapters.ports.domain.order`; associated with `ordering.adapters.rest`.
- `orderRecord` — Postgres/driven/outbound JPA entity; `maps → ordering.adapters.ports.domain.order`; associated with `ordering.adapters.ordersAdapter`.

Being top-level (outside the `ordering` container) keeps them out of the nested-ring views entirely.

## New view: `data-mapping`

- `title`: "Data Mapping (boundaries)"
- `include`: the two boundary objects, `ordering.adapters.ports.domain.order` (optionally `line`/`total`), `ordering.adapters.rest`, `ordering.adapters.ordersAdapter`, and the relevant ports (`placeOrder`, `orderRepo`).
- `layout`: `layered` — initial proximity comes from `maps`/association adjacency (BFS); the human refines the arrangement in draw.io and reverse-sync preserves it (Bausteinsicht's refine-and-sync philosophy).
- The ring views are unchanged.

## Template / styling

- `boundaryObject` style — a UML-class-like rectangle in a distinct colour (clearly "data", not a component), with a `«dto»`-style hint.
- `maps` connector — dashed, open arrowhead (via the `dashed: true` relationship kind + connector template).

## Constraints

- HEX-001..007 are unchanged. Because the new `kind`/relationship are not referenced by any HEX selector, HEX-001..006 are unaffected.
- **HEX-007 (`no-circular-dependency`) has no kind filter**, so the new `maps`/association edges participate in the cycle check. Directions are chosen to point "inward" (`adapter → boundaryObject → domainObject`) with no domain→adapter return path, keeping the graph acyclic. **Verify with `lint`** during implementation; if a cycle appears, adjust direction (do not add a pad/exclusion that hides a real cycle).

## CLI management (dogfooding)

The whole feature is already manageable with the existing `add` CLI (no new CLI work) — this both satisfies the "CLI / LLM automation" quality goal and dog­foods the comment-preserving patch save (ADR-011):

| Step | Command |
|---|---|
| kind | `bausteinsicht add specification element boundaryObject --notation "Boundary Object" --description "…"` |
| `maps` kind (dashed) | `bausteinsicht add specification relationship maps --notation maps --dashed --description "…"` |
| element | `bausteinsicht add element --id orderRecord --kind boundaryObject --title "OrderRecord" --technology "JPA/Hibernate" …` |
| `maps` edge | `bausteinsicht add relationship --from orderRecord --to ordering.adapters.ports.domain.order --kind maps --label "maps"` |
| association | `bausteinsicht add relationship --from ordering.adapters.ordersAdapter --to orderRecord --kind uses --label "produces"` |
| view | `bausteinsicht add view data-mapping --title "Data Mapping (boundaries)" --include …` |

**Implementation approach (decided): dogfood via the CLI** — create the new kind/relationship/elements/view with the `add` commands above, and document those exact commands in the README. The `data-mapping` view uses the default `layered` layout, so the missing `add view --layout` flag is not a blocker. After the CLI writes the model, re-check the example's hand-formatting/comments and tidy if the patch save disturbed them.

## Documentation

- New README section in `examples/hexagonal-ddd/README.adoc`: the boundary-mapping pattern (domain-in / adapter-maps-out, where the mapper sits), why the example ships only two exemplars, and the seam rationale (why data objects are NOT modelled at class granularity). Links to the `data-mapping` view **and shows the `add` CLI commands** that create the boundary objects (demonstrates CLI/LLM manageability).

## Testing

- The `add` commands themselves are the first test — they succeed and produce a `validate`-clean model (dogfooding).
- `bausteinsicht validate` on the example → valid.
- `bausteinsicht lint` on the example → all constraints pass (explicitly confirms HEX-007 stays green with the new edges).
- Render the `data-mapping` page to confirm the two boundary objects, their `maps` edges and adapter associations are shown and legible.
- No new CLI command/flag/bug-fix, so no new e2e scenario is strictly required by the merge policy; if the nested-layout e2e harness is a convenient home, a tiny "example validates + lints" assertion may be added.

## Follow-ups (out of this design)

- ADR for the boundary-data modelling convention (deferred by decision).
- Possibly a second/third exemplar (payment/Stripe boundary) if the two prove too thin.
- Apply the same treatment to `examples/hexagonal-jmolecules` later.
- **CLI completeness (relates to #392):** `add view` has no `--layout` flag and `add element` has no `--metadata`/`--tags` flag. Neither blocks this feature, but they are real gaps for full CLI/LLM manageability of non-default layouts and tagged/metadata elements. (Concretely surfaced here: `add element` could not tag the new `payment` aggregate, so it rendered green instead of white until the tag was added by hand.)

## Ideas surfaced during this work (candidate issues — general tool, not this example)

These came up while building the boundary/payment data view. Each is a separate feature, worth its own issue.

- **Presentation / view-composition constraints.** Today constraints check the *model's* relationships/structure (e.g. HEX-003/004: only driven adapters may reach `datastore`/`externalSystem`). There is no rule about *what a view may contain*. A new constraint type could warn when a view includes kinds that do not belong in it — e.g. "the `data-mapping` view should not include `datastore`/`externalSystem`" (infrastructure). This would give a `lint`/`validate` warning when a user or LLM adds infra to the wrong view. Fits the "fitness functions" goal; distinct from the model-level rules.
- **"New element" marker has no acknowledgment, and is style-destructive.** Incrementally-added elements get a red-dashed marker (`strokeColor=#FF0000;dashed=1`) meaning "please position me", but:
  - Bausteinsicht never auto-clears it and cannot tell when the user has positioned/approved the element (it clears only on manual edit or `sync --relayout`, which discards positions).
  - The marker *overwrites* the element's `strokeColor`, so reverting it by hand is fiddly (the original kind/tag colour is lost and must be re-typed or copied from a sibling), and any *deliberate* custom colour the user gave the element would be clobbered by a naive "reset to kind default" finalize.

  Design directions (roughly in increasing effort):
  1. **Warn on leftover markers.** Simplest first step: `sync`/`validate`/`lint` reports "N elements are still marked as new/unplaced" (detect the `#FF0000;dashed=1` cells). No redesign needed; gives the missing feedback loop the user asked about.
  2. **Non-destructive marker.** Track the "new/unplaced" state as a separate attribute (e.g. `bausteinsicht_new="true"` on the `<object>`) or an overlay, *not* by hijacking `strokeColor`. Then the base/kind/tag style — including a deliberate custom colour — is preserved underneath.
  3. **Finalize command.** A `bausteinsicht finalize` (or `sync --accept-new`) that clears the "new" state without touching colours or positions; optionally auto-clear on the next sync once an element has moved from its default position. With (2) in place this is trivial and safe.
- **Legend with real symbols (not just colour swatches).** The generated legend lists kinds with a small colour square. It could instead render each kind's *actual shape* (hexagon for ports, component for services, cylinder for datastore, …) so the legend matches the diagram vocabulary.
- **UML stereotypes on elements.** Optionally render the kind's notation as a UML stereotype above the title (`«Adapter»`, `«Port»`, `«Aggregate»`), driven from the template/kind — a familiar, self-documenting cue.
- **Technology catalogue / picker.** An optional curated set of technologies (with icons?) to choose from when authoring, instead of free-text `technology` strings — an addon for consistency and nicer rendering. (Technology is empty by default; the example sets it deliberately.)
- **`sync --debug` / `--explain` diagnostics.** Today the only signal about template/layout resolution is the `no template style for kind: X` / `no boundary template for kind: X` warnings; `--verbose` adds little. A debug mode should make sync *self-diagnosing*: which template file was loaded (local `template.drawio` vs. embedded default), and per element the resolved `kind → template shape + applied tags + final style`; per `layered` view the computed tier order (`kind → row`); and per view the layout mode used. This would immediately answer the recurring questions from this session ("is my template being picked up?", "why is the row order like this?", "why is that element still the old colour?").
- **Finer / per-view `layered` ordering.** Today `layered` orders rows top-to-bottom by the *global* kind order in `specification.elements`, and all elements of a kind share one alphabetically-sorted row; it ignores `metadata.side`. Two enhancements would help layered *data*/component views: (1) let `layered` honour `metadata.side` (or an explicit per-element `tier` hint) so a single kind can be split into a top/bottom row (e.g. inbound vs. outbound boundary objects) without inventing extra kinds; (2) allow a *per-view* layer ordering so different views can order the same kinds differently (inbound-top / core-centre / outbound-bottom in one view, something else in another). Documented current behaviour: `src/docs/spec/05_sync_specification.adoc` Layout Modes TIP.

## Decisions (confirmed)

- Approach **B** (lightweight model, separate `data-mapping` view).
- Kind name **`boundaryObject`**, **single** data kind (domain side reuses existing kinds).
- **Two** exemplars (`placeOrderRequest`, `orderRecord`).
- **Dogfood via the CLI** and document the commands in the README.
- **ADR deferred.**
- **Branch:** implement **on the current `feature/hexagonal-ddd-example` branch (#571)** — the user opted to build it here rather than stacking/waiting.
