# Substack Creator Newsletter Engine — Definition of Done

## Scope

### In Scope
- Pure React frontend (no backend) for brand-driven Substack content creation
- Client-side Gemini LLM integration (3.1 Flash, 3.1 Pro w/ extended thinking, 2.5 Flash Lite for tests)
- IndexedDB persistence for config, drafts, sessions, and post history
- Setup flow: API key, company, voice, guardrails
- Dashboard with post history, drafts, and navigation
- Trending Topics with search-grounded research and synthesized prompts
- New Post pipeline: Topic → Research → Outline → Write/Edit/Guardrails → Complete
- Demo mode: replay cached sessions with prefilled UI; ships with one bundled P&G session
- Production mode: record all sessions for later replay
- Structured JSON output with retry/backoff for all LLM calls
- Integration tests (canned data), smoke tests (live Flash Lite), manual test option (real models)
- Deployment readiness artifacts for GitHub -> Railway, validated via code review
- Explicit non-deployment policy for acceptance: no live deploy execution
- Visual design matching Substack look and feel

### Out of Scope
- Backend server or API layer
- Multi-company workspace support (v01 is single workspace)
- RAG or document retrieval beyond search grounding
- Manual pause/edit/rerun controls between Write/Edit/Guardrails cycles
- Substack API publishing integration
- Executing a live deployment to Railway (or any hosting platform) during DoD verification

### Assumptions
- User has a valid Gemini API key
- Browser supports IndexedDB
- A reviewer can inspect deployment configuration/docs to validate deployability
- Node.js/npm toolchain available for build and test

## Deliverables

| Artifact | Location | Description |
|----------|----------|-------------|
| React application | `src/` | Complete SPA source code |
| Build output | `dist/` or `build/` | Production-ready static assets |
| Bundled demo session | `src/demo/` or equivalent | Canned P&G session data (config + post) for out-of-box demo mode |
| Test suite | `src/__tests__/` or colocated | Integration tests (canned), smoke tests (live Flash Lite) |
| Browser test evidence bundle | `.ai/test-evidence/latest/` | Scenario evidence artifacts plus manifest mapping each `IT-*` to status and artifact paths/types |
| Deployment config | `railway.json` / `Procfile` or equivalent | GitHub -> Railway deployment-ready config and docs, validated by review only (not executed) |
| `package.json` | project root | Dependencies, scripts for build, test, dev |

## Evidence Contract

- Test runs must write `.ai/test-evidence/latest/manifest.json`.
- Manifest must include one entry per executed `IT-*` scenario, with:
  - scenario ID
  - pass/fail status
  - artifact list (path + artifact type)
  - short summary of key assertions
- UI scenarios (`IT-2` through `IT-8`, `IT-11`, `IT-12`) require screenshots.
- Browser scenarios must include trace/log artifacts that capture console errors/unhandled exceptions (or explicit `none` when absent).
- Failing scenarios must still emit best-effort artifacts and a failing manifest entry.
- Exit code alone is insufficient evidence for browser acceptance.

## Acceptance Criteria

### 1 — Build & Deploy

| ID | Criterion | Covered by |
|----|-----------|------------|
| AC-1.1 | `npm install && npm run build` exits 0 and produces static assets | IT-1 |
| AC-1.2 | App loads in a browser at its served URL without console errors | IT-2, IT-3 |
| AC-1.3 | Repository contains deployment-ready GitHub -> Railway config/docs that pass deployment-readiness code review and correctly reference build output | IT-1 |
| AC-1.4 | DoD verification performs no live deployment; deployment validation is review/static-check only | IT-1 |

### 2 — Persistence (IndexedDB)

| ID | Criterion | Covered by |
|----|-----------|------------|
| AC-2.1 | API key is stored in IndexedDB and persists across page reloads | IT-3 |
| AC-2.2 | Company, voice, and guardrails config persist in IndexedDB across reloads | IT-3 |
| AC-2.3 | Completed posts are saved to IndexedDB and appear in dashboard history | IT-5 |
| AC-2.4 | Draft state persists in IndexedDB and can be resumed | IT-6 |
| AC-2.5 | Production sessions (all inputs, LLM responses, intermediate state) are recorded in IndexedDB | IT-5, IT-7 |
| AC-2.6 | "Reset everything" deletes all IndexedDB data | IT-8 |
| AC-2.7 | Saved posts and recorded sessions persist citation-to-source attribution mappings so each citation can be resolved to original source metadata after reload/reopen | IT-5, IT-7 |

### 3 — LLM Integration

| ID | Criterion | Covered by |
|----|-----------|------------|
| AC-3.1 | All LLM calls use structured output mode (JSON enforced to schema) | IT-4, IT-5 |
| AC-3.2 | Malformed LLM responses trigger retry with error feedback and intelligent backoff | IT-9 |
| AC-3.3 | Flash is used for small/fast tasks (research, trending topic fetches) | IT-4, IT-5 |
| AC-3.4 | Pro with extended thinking is used for important tasks (confirmations, outline, write cycles, trending synthesis) | IT-3, IT-4, IT-5 |
| AC-3.5 | Test mode substitutes Flash Lite for all models | IT-10 |
| AC-3.6 | API key is sent client-side; no server-side proxy | IT-5 |

### 4 — Setup Flow

| ID | Criterion | Covered by |
|----|-----------|------------|
| AC-4.1 | First app launch navigates to Settings page | IT-2 |
| AC-4.2 | Non-first launch navigates to Dashboard | IT-3 |
| AC-4.3 | Setup options are parallel — completable in any order | IT-3 |
| AC-4.4 | Each incomplete setup item shows an icon indicating empty state | IT-2 |
| AC-4.5 | API key step collects and stores the key | IT-3 |
| AC-4.6 | Company step accepts rich input, then Pro confirms; back button returns to input | IT-3 |
| AC-4.7 | Voice step accepts rich input, then Pro confirms; back button returns to input | IT-3 |
| AC-4.8 | Guardrails step accepts rich input, then Pro confirms; back button returns to input | IT-3 |
| AC-4.9 | Settings page becomes navigable to Dashboard once all required items complete | IT-3 |
| AC-4.10 | Rich input control accepts typed/pasted text, uploaded documents, and links | IT-3 |

### 5 — Dashboard

| ID | Criterion | Covered by |
|----|-----------|------------|
| AC-5.1 | Dashboard displays a prominent "New Post" button | IT-5 |
| AC-5.2 | Dashboard displays a prominent "Trending Topics" button | IT-4 |
| AC-5.3 | Dashboard provides access to Settings page | IT-3 |
| AC-5.4 | Dashboard lists past posts and drafts | IT-5, IT-6 |
| AC-5.5 | Past posts can be viewed in a post viewer | IT-5 |
| AC-5.6 | Drafts can be resumed from the dashboard | IT-6 |

### 6 — Trending Topics

| ID | Criterion | Covered by |
|----|-----------|------------|
| AC-6.1 | Opening Trending Topics launches parallel Flash search-grounded research calls | IT-4 |
| AC-6.2 | Results build a deterministic trend visualization as they arrive | IT-4 |
| AC-6.3 | Pro synthesizes sources into 3 writing prompts | IT-4 |
| AC-6.4 | Selecting a prompt navigates to New Post with topic prefilled | IT-4 |

### 7 — New Post Flow

| ID | Criterion | Covered by |
|----|-----------|------------|
| AC-7.1 | Topic step presents the rich input control and a "Research" button | IT-5 |
| AC-7.2 | Research step uses Flash with search grounding; sources display as headline + snippet | IT-5 |
| AC-7.3 | Each research source carries structured metadata (URL, title, author, publication date) | IT-5 |
| AC-7.4 | User can highlight interesting sources and delete irrelevant ones | IT-5 |
| AC-7.5 | Outline is generated by Pro in one shot from all gathered materials | IT-5 |
| AC-7.6 | User can accept outline or go back to Research | IT-5 |
| AC-7.7 | Write cycle 1 (Write): Pro one-shots article without guardrails, with inline citations | IT-5 |
| AC-7.8 | Write cycle 2 (Edit): Pro revises for style guide alignment | IT-5 |
| AC-7.9 | Write cycle 3 (Guardrails): receives only guardrails doc + post, fixes issues | IT-5 |
| AC-7.10 | All three write cycles run automatically with no manual controls between them | IT-5 |
| AC-7.11 | Each write cycle is visible to the user as it progresses | IT-5 |
| AC-7.12 | Complete step shows formatted post with footnotes — each citation has source title and link | IT-5 |
| AC-7.13 | Final post is saved as Markdown and displayed in a formatted Markdown viewer | IT-5 |
| AC-7.14 | Citations are best-effort: source-derived claims are cited, unsourced generic/opinion statements are allowed without citation | IT-5 |
| AC-7.15 | Attribution lineage is preserved end-to-end: citations in final output resolve to retained research sources with original metadata (URL, title, author, publication date) | IT-5 |

### 8 — Demo Mode

| ID | Criterion | Covered by |
|----|-----------|------------|
| AC-8.1 | Demo mode shows a session picker listing all recorded sessions | IT-7 |
| AC-8.2 | Selecting a session replays it through production code path | IT-7 |
| AC-8.3 | Text fields prefill instantly with a subtle fade-in | IT-7 |
| AC-8.4 | Attachments appear a moment after page loads | IT-7 |
| AC-8.5 | The button that was clicked next becomes highlighted | IT-7 |
| AC-8.6 | User clicks through each step seeing what a real session looks like | IT-7 |
| AC-8.7 | Cache miss displays an error; does not fall back to live API | IT-11 |
| AC-8.8 | App ships with one bundled P&G session so demo mode works out of the box | IT-7 |
| AC-8.9 | Bundled session includes cached configuration (first-run setup) | IT-7 |

### 9 — Visual Design

| ID | Criterion | Covered by |
|----|-----------|------------|
| AC-9.1 | Font family, colors, and visual styling resemble Substack | IT-12 |
| AC-9.2 | Rich input is a textarea with attach and link toolbar at the bottom | IT-3 |
| AC-9.3 | Multi-step flows show horizontal dots: accent = current, green = done, outlined = future | IT-5 |
| AC-9.4 | Content blocks, source results, confirmations, and previews render in the same card primitive | IT-5 |
| AC-9.5 | Progress uses horizontal bars with accent color fill animating left-to-right; no spinners or skeletons | IT-5 |
| AC-9.6 | Final post preview is serif with numbered footnotes and linked sources — looks like a real Substack post | IT-12 |

### 10 — Testing Infrastructure

| ID | Criterion | Covered by |
|----|-----------|------------|
| AC-10.1 | Integration tests run with canned LLM data and pass without network access | IT-10 |
| AC-10.2 | Smoke tests run with live Flash Lite calls after integration tests pass | IT-10 |
| AC-10.3 | Manual test option exists to use real model configuration (Flash + Pro) | IT-10 |
| AC-10.4 | Structured retry logic handles nondeterminism in Flash Lite smoke tests | IT-9, IT-10 |

### 11 — Browser Evidence Contract

| ID | Criterion | Covered by |
|----|-----------|------------|
| AC-11.1 | Every executed integration scenario writes a manifest entry in `.ai/test-evidence/latest/manifest.json` with status and artifact metadata | IT-1 through IT-12 |
| AC-11.2 | UI/browser scenarios include screenshots and trace/log evidence, not only pass/fail exit code | IT-2, IT-3, IT-4, IT-5, IT-6, IT-7, IT-8, IT-11, IT-12 |
| AC-11.3 | Browser console/unhandled error summaries are captured per browser scenario | IT-2, IT-3, IT-4, IT-5, IT-6, IT-7, IT-8, IT-11, IT-12 |
| AC-11.4 | Failing scenarios still produce best-effort evidence artifacts and manifest entries | IT-9, IT-11 |

## User-Facing Message Inventory

| ID | Message surface | Trigger condition | Covered by |
|----|----------------|-------------------|------------|
| MSG-1 | Settings page with status icons for incomplete items | First launch or navigate to Settings | IT-2 |
| MSG-2 | API key input field and save confirmation | Open API key setup | IT-3 |
| MSG-3 | Rich input control (textarea + attach/link toolbar) | Any input step (company, voice, guardrails, topic) | IT-3, IT-5 |
| MSG-4 | Company confirmation display (Pro-generated) | Submit company identity | IT-3 |
| MSG-5 | Company confirmation back button | Viewing company confirmation | IT-3 |
| MSG-6 | Voice confirmation display (Pro-generated) | Submit voice definition | IT-3 |
| MSG-7 | Voice confirmation back button | Viewing voice confirmation | IT-3 |
| MSG-8 | Guardrails confirmation display (Pro-generated) | Submit guardrails definition | IT-3 |
| MSG-9 | Guardrails confirmation back button | Viewing guardrails confirmation | IT-3 |
| MSG-10 | "Reset Everything" option (subtle) with confirmation dialog | Click reset on Settings | IT-8 |
| MSG-11 | Dashboard "New Post" button (prominent) | Dashboard loaded | IT-5 |
| MSG-12 | Dashboard "Trending Topics" button (prominent) | Dashboard loaded | IT-4 |
| MSG-13 | Dashboard Settings navigation option | Dashboard loaded | IT-3 |
| MSG-14 | Past posts and drafts list | Dashboard loaded with history | IT-5, IT-6 |
| MSG-15 | Post viewer display | Click to view a past post | IT-5 |
| MSG-16 | Trending research results (headline + snippet streaming in) | Open Trending Topics | IT-4 |
| MSG-17 | Deterministic trend visualization | Research results return | IT-4 |
| MSG-18 | Three Pro-generated writing prompts | Trending synthesis completes | IT-4 |
| MSG-19 | Topic input with "Research" button | Start New Post | IT-5 |
| MSG-20 | Research source cards (headline + snippet + metadata) | Research results return | IT-5 |
| MSG-21 | Research highlight/delete controls per source | Research results displayed | IT-5 |
| MSG-22 | Outline display with accept/back options | Outline generated | IT-5 |
| MSG-23 | Write cycle progress — cycle 1 (Write) visible | Write cycle begins | IT-5 |
| MSG-24 | Write cycle progress — cycle 2 (Edit) visible | Edit cycle begins | IT-5 |
| MSG-25 | Write cycle progress — cycle 3 (Guardrails) visible | Guardrails cycle begins | IT-5 |
| MSG-26 | Horizontal progress bar (accent, animating left-to-right) | Any LLM operation in progress | IT-5 |
| MSG-27 | Step indicator dots (accent=current, green=done, outlined=future) | Any multi-step flow | IT-5 |
| MSG-28 | Completed post with serif formatting, numbered footnotes, linked sources | Post complete | IT-5, IT-12 |
| MSG-29 | Demo session picker | Enter demo mode | IT-7 |
| MSG-30 | Demo replay — text prefills with fade-in | Replaying a session step | IT-7 |
| MSG-31 | Demo replay — attachments appear after brief delay | Replaying a step with attachments | IT-7 |
| MSG-32 | Demo replay — next button highlighted | Replaying a session step | IT-7 |
| MSG-33 | Demo cache-miss error (no API fallback) | Cache miss during demo replay | IT-11 |
| MSG-34 | Card primitive (content blocks, source results, confirmations, previews) | Throughout the app | IT-5 |
| MSG-35 | Reset confirmation dialog | Click reset, before confirming | IT-8 |

## Integration Test Scenarios

| ID | Scenario | Steps | Verification |
|----|----------|-------|--------------|
| IT-1 | **Build and deployment-readiness review (no live deploy)** | 1. Run `npm install` → exits 0 2. Run `npm run build` → exits 0, `dist/` or `build/` contains `index.html` and JS assets 3. Verify deployment config file(s) exist (`railway.json` / `Procfile` or equivalent) and reference the built static output 4. Perform deployment-readiness code review: verify build/start wiring and required env placeholders are documented, with no missing repository configuration needed for GitHub -> Railway handoff 5. Verify this scenario and all DoD scenarios do not execute live deployment commands | Build/review checks pass and manifest includes `IT-1` with static evidence paths (build logs and review checklist artifact) |
| IT-2 | **First-run routing to Settings** | 1. Clear all IndexedDB data 2. Serve app and load in browser 3. Verify app navigates to Settings page (not Dashboard) 4. Verify each setup item (API key, company, voice, guardrails) shows incomplete icon 5. Verify "Reset Everything" is present but visually subtle | Manifest includes `IT-2=pass`, screenshot(s) proving first-run Settings state, and browser console/unhandled-error summary |
| IT-3 | **Complete setup flow (all steps, any order)** | 1. Clear IndexedDB, serve app, load in browser → lands on Settings 2. Complete guardrails first: enter text via rich input (verify textarea + toolbar), submit → verify Pro confirmation displays, click back → returns to input, resubmit → confirm, returns to Settings 3. Complete API key: enter key, save → verify persisted 4. Complete voice: enter via rich input, submit → verify Pro confirmation + back button, confirm → returns to Settings 5. Complete company: enter via rich input, submit → verify Pro confirmation + back button, confirm → returns to Settings 6. Verify all status icons now show complete 7. Navigate to Dashboard → verify Dashboard loads 8. Reload page → verify app goes to Dashboard (not Settings) 9. Navigate to Settings from Dashboard → verify settings accessible 10. Verify API key, company, voice, guardrails data persisted in IndexedDB | Manifest includes `IT-3=pass`, screenshots for completed settings + dashboard, and artifact evidence of persisted IndexedDB keys/values plus console summary |
| IT-4 | **Trending Topics end-to-end** | 1. Set up app with completed config (canned) 2. Navigate to Dashboard → verify "Trending Topics" button is prominent 3. Click Trending Topics 4. Verify Flash search-grounded research calls launch (canned responses) 5. As results return, verify headline + snippet cards appear 6. Verify deterministic trend visualization renders 7. Verify Pro synthesizes 3 writing prompts 8. Select one prompt → verify navigation to New Post with topic prefilled 9. Verify back button from Trending Topics returns to Dashboard | Manifest includes `IT-4=pass`, screenshots of trend cards/visualization/prompts, and trace/log evidence for prompt-to-New-Post navigation plus console summary |
| IT-5 | **New Post end-to-end (Topic → Complete)** | 1. Set up app with completed config (canned) 2. Dashboard loads → verify "New Post" button is prominent 3. Click New Post → verify Topic step with rich input control + "Research" button 4. Verify step indicator dots: step 1 = accent, steps 2–5 = outlined 5. Enter topic, click Research 6. Verify Flash search-grounded results arrive as source cards (headline + snippet + metadata: URL, title, author, date) 7. Highlight one source, delete another → verify UI updates 8. Proceed → verify Outline generated by Pro (one shot) 9. Verify back button from Outline returns to Research 10. Accept outline → verify Write cycle begins 11. Verify cycle 1 (Write) progress visible with horizontal bar (accent, left-to-right animation) — no spinners 12. Verify cycle 2 (Edit) progress visible 13. Verify cycle 3 (Guardrails) progress visible 14. Verify all 3 cycles run automatically (no manual controls between) 15. Verify Complete step: formatted post, serif font, numbered footnotes, each citation has source title + link 16. Verify each citation resolves to a retained research source and preserves original metadata (URL, title, author, publication date) captured at Research step 17. Verify post saved as Markdown, displayed in Markdown viewer, and includes attribution mapping data needed to resolve citations 18. Return to Dashboard → verify post appears in history 19. Click post in history → verify post viewer displays it with attribution still resolvable 20. Reload app, reopen same post from history, verify citation-to-source attribution mappings are unchanged | Manifest includes `IT-5=pass`, multi-step screenshots (Topic/Research/Outline/Write cycles/Complete), and explicit artifact proving citation-to-source attribution persistence across reload |
| IT-6 | **Draft save and resume** | 1. Set up app with completed config 2. Start New Post → enter topic → advance to Research 3. Close or navigate away mid-flow 4. Return to Dashboard → verify draft appears in drafts list 5. Click resume → verify post flow resumes from saved state with prior inputs intact | Manifest includes `IT-6=pass`, screenshots showing draft listed + resumed state, and evidence that draft persistence keys are present |
| IT-7 | **Demo mode replay (bundled P&G session)** | 1. Fresh app with bundled demo data (no user config needed) 2. Navigate to Demo mode → verify session picker displays available sessions 3. Verify bundled P&G session is listed 4. Select it → verify replay begins through production code path 5. Verify setup steps replay: text fields prefill with subtle fade-in 6. Verify attachments appear after a brief delay 7. Verify next-step button becomes highlighted 8. Click through each step → verify full flow replays (config setup through post completion) 9. Verify replay ends and returns to Dashboard | Manifest includes `IT-7=pass`, screenshots for picker + replay states, and trace/log evidence confirming replay path from cached session data |
| IT-8 | **Reset everything** | 1. Set up app with completed config, at least one post, and one draft 2. Navigate to Settings → verify "Reset Everything" option is present and visually subtle 3. Click Reset → verify confirmation dialog appears 4. Cancel → verify no data deleted 5. Click Reset again → Confirm → verify all IndexedDB data wiped 6. Verify app returns to first-run state (Settings with all items showing incomplete) | Manifest includes `IT-8=pass`, screenshots before/after reset, and evidence artifact proving IndexedDB stores are empty after confirmation |
| IT-9 | **LLM structured output retry and backoff** | 1. Mock LLM endpoint to return malformed JSON on first call, valid structured JSON on retry 2. Trigger an LLM call (e.g., company confirmation) 3. Verify first call fails, retry fires with error feedback included in prompt 4. Verify second call succeeds with valid response 5. Mock endpoint to return malformed JSON repeatedly → verify intelligent backoff (increasing delays between retries) 6. Verify app does not crash; surfaces error gracefully after max retries | Test exits 0 and manifest includes `IT-9` with retry/backoff log artifacts showing malformed-response recovery and terminal failure behavior |
| IT-10 | **Test infrastructure: integration, smoke, and manual modes** | 1. Run integration test suite → verify all tests pass with canned data and no network calls 2. Verify test config substitutes Flash Lite for all models 3. Run smoke tests with live Flash Lite → verify structured retry handles nondeterminism 4. Verify manual test option flag/config exists to use real Flash + Pro 5. Verify smoke tests only run after integration tests pass (test ordering) | `npm test` exits 0 and manifest includes `IT-10` with artifacts proving mode selection and ordering (integration before smoke) |
| IT-11 | **Demo mode cache-miss error** | 1. Load app with a demo session that has an intentional cache gap (missing one LLM response) 2. Start demo replay 3. Advance to the step with the cache miss 4. Verify error message displays indicating cache miss 5. Verify app does NOT fall back to live API calls 6. Verify dismiss returns to demo session picker | Manifest includes `IT-11` (pass or fail), screenshot of cache-miss error state, and network/trace evidence that no live API fallback call was made |
| IT-12 | **Visual design verification** | 1. Load app in browser 2. Verify font family matches Substack styling (serif for post content) 3. Verify color palette aligns with Substack look and feel 4. Verify final post preview: serif text, numbered footnotes, linked source titles — visually resembles a real Substack post 5. Verify no spinners or skeleton loaders anywhere — only horizontal accent-color progress bars 6. Verify card components have clean spacing, no shadows 7. Capture screenshots of: Settings page, Dashboard, Trending Topics results, New Post step indicators, Write cycle progress, Complete post view | Manifest includes `IT-12=pass` with required screenshot set and a short visual-review checklist artifact |

## Crosscheck

### Per scenario

| Scenario | Exercises delivered artifact? | Automatable? | Bounded? | Proportional? | Independent? | Crosses AC groups? |
|----------|------------------------------|-------------|----------|---------------|-------------|-------------------|
| IT-1 | Yes — builds the app | Yes (plus review checklist) | Yes (5 steps) | Yes | Yes | 1, 11 |
| IT-2 | Yes — loads in browser | Yes | Yes (5 steps) | Yes | Yes | 4, 9, 11 |
| IT-3 | Yes — loads in browser | Yes (canned LLM) | Yes (10 steps) | Yes | Yes | 2, 3, 4, 5, 9, 11 |
| IT-4 | Yes — loads in browser | Yes (canned LLM) | Yes (9 steps) | Yes | Yes | 3, 5, 6, 11 |
| IT-5 | Yes — loads in browser | Yes (canned LLM) | Yes (20 steps) | Yes | Yes | 2, 3, 5, 7, 9, 11 |
| IT-6 | Yes — loads in browser | Yes | Yes (5 steps) | Yes | Yes | 2, 5, 11 |
| IT-7 | Yes — loads in browser | Yes | Yes (9 steps) | Yes | Yes | 8, 11 |
| IT-8 | Yes — loads in browser | Yes | Yes (6 steps) | Yes | Yes | 2, 4, 11 |
| IT-9 | Yes — tests LLM client | Yes (mocked) | Yes (6 steps) | Yes | Yes | 3, 10, 11 |
| IT-10 | Yes — runs test suite | Yes | Yes (5 steps) | Yes | Yes | 3, 10, 11 |
| IT-11 | Yes — loads in browser | Yes | Yes (6 steps) | Yes | Yes | 8, 11 |
| IT-12 | Yes — loads in browser | Yes (screenshot comparison) | Yes (7 steps) | Yes | Yes | 9, 11 |

### Per AC — all covered

| AC | Covered by |
|----|------------|
| AC-1.1 | IT-1 |
| AC-1.2 | IT-2, IT-3 |
| AC-1.3 | IT-1 |
| AC-1.4 | IT-1 |
| AC-2.1 | IT-3 |
| AC-2.2 | IT-3 |
| AC-2.3 | IT-5 |
| AC-2.4 | IT-6 |
| AC-2.5 | IT-5, IT-7 |
| AC-2.6 | IT-8 |
| AC-2.7 | IT-5, IT-7 |
| AC-3.1 | IT-4, IT-5 |
| AC-3.2 | IT-9 |
| AC-3.3 | IT-4, IT-5 |
| AC-3.4 | IT-3, IT-4, IT-5 |
| AC-3.5 | IT-10 |
| AC-3.6 | IT-5 |
| AC-4.1 | IT-2 |
| AC-4.2 | IT-3 |
| AC-4.3 | IT-3 |
| AC-4.4 | IT-2 |
| AC-4.5 | IT-3 |
| AC-4.6 | IT-3 |
| AC-4.7 | IT-3 |
| AC-4.8 | IT-3 |
| AC-4.9 | IT-3 |
| AC-4.10 | IT-3 |
| AC-5.1 | IT-5 |
| AC-5.2 | IT-4 |
| AC-5.3 | IT-3 |
| AC-5.4 | IT-5, IT-6 |
| AC-5.5 | IT-5 |
| AC-5.6 | IT-6 |
| AC-6.1 | IT-4 |
| AC-6.2 | IT-4 |
| AC-6.3 | IT-4 |
| AC-6.4 | IT-4 |
| AC-7.1 | IT-5 |
| AC-7.2 | IT-5 |
| AC-7.3 | IT-5 |
| AC-7.4 | IT-5 |
| AC-7.5 | IT-5 |
| AC-7.6 | IT-5 |
| AC-7.7 | IT-5 |
| AC-7.8 | IT-5 |
| AC-7.9 | IT-5 |
| AC-7.10 | IT-5 |
| AC-7.11 | IT-5 |
| AC-7.12 | IT-5 |
| AC-7.13 | IT-5 |
| AC-7.14 | IT-5 |
| AC-7.15 | IT-5 |
| AC-8.1 | IT-7 |
| AC-8.2 | IT-7 |
| AC-8.3 | IT-7 |
| AC-8.4 | IT-7 |
| AC-8.5 | IT-7 |
| AC-8.6 | IT-7 |
| AC-8.7 | IT-11 |
| AC-8.8 | IT-7 |
| AC-8.9 | IT-7 |
| AC-9.1 | IT-12 |
| AC-9.2 | IT-3 |
| AC-9.3 | IT-5 |
| AC-9.4 | IT-5 |
| AC-9.5 | IT-5 |
| AC-9.6 | IT-12 |
| AC-10.1 | IT-10 |
| AC-10.2 | IT-10 |
| AC-10.3 | IT-10 |
| AC-10.4 | IT-9, IT-10 |
| AC-11.1 | IT-1, IT-2, IT-3, IT-4, IT-5, IT-6, IT-7, IT-8, IT-9, IT-10, IT-11, IT-12 |
| AC-11.2 | IT-2, IT-3, IT-4, IT-5, IT-6, IT-7, IT-8, IT-11, IT-12 |
| AC-11.3 | IT-2, IT-3, IT-4, IT-5, IT-6, IT-7, IT-8, IT-11, IT-12 |
| AC-11.4 | IT-9, IT-11 |

### Per message — all covered

All 35 message surfaces (MSG-1 through MSG-35) are mapped to at least one integration test scenario. See the "Covered by" column in the Message Inventory table.

### Overall

- At least one scenario loads the app in a browser: IT-2, IT-3, IT-4, IT-5, IT-6, IT-7, IT-8, IT-11, IT-12
- Every user-facing message is triggered and validated by at least one scenario
- All 11 AC groups are covered across scenarios
