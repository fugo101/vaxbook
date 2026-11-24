# Accessibility

VaxBook aims to be usable by people with disabilities, including people who use a keyboard only, a screen reader, zoom or high contrast settings (forced colors).

## Target

The target is [WCAG 2.2](https://www.w3.org/TR/WCAG22/) level AA (W3C Recommendation, 12 December 2024, latest published edition). The project owner set this target on 2026-11-24. It is recorded as a quality attribute in the [SAD](docs/sad.md) §1.

It applies to:

- the web app (`apps/web`, the PWA), for guests and signed-in users, including the print page (F09);
- the admin area (F11, [ADR-0008](docs/adr/0008-admin-behind-cloudflare-access.md));
- the emails VaxBook sends (the sign-in code F01.1, the notice to the old address on an email change F01.6, and the reminders F08), as far as WCAG applies to them. WCAG is written for web content, so for email it is followed as good practice, not claimed.

Third-party pages and widgets, namely the Cloudflare Access login in front of the admin area and the Turnstile widget, are outside the project's control and are not covered by the target. Barriers found in them are listed under Known limitations.

## Status

Nothing user-facing is built yet: `apps/` holds only the API. VaxBook therefore makes **no conformance claim**, and no audit has been done. The target is how the web app will be built and reviewed. An accessibility statement for the live app, with the environments it was tested in, can be added once the app ships and has been evaluated.

## What the target means here

These are the points that matter most for this product. They add to WCAG; they do not replace it.

- **Color is never the only signal** (SC 1.4.1). Every bucket keeps its text label wherever it appears, including emails and the print page, for example "Quá hạn / cần tiêm bù" ("Overdue / catch-up needed") and "Chưa rõ" ("Unclear"). A colored emoji or dot alone is not enough. Status indicators the app draws itself (dots, borders, icons) also differ in shape or text, and meet 3:1 contrast (SC 1.4.11). Indicators the app draws stay visible in forced-colors mode (they do not rely on background color alone). 🟠 "Đến hạn" ("Due") and 🟡 "Sắp tới" ("Upcoming") are the pair most at risk.
- **Status emoji are decorative.** Next to a text label, a status emoji (🔴 🟠 🟡 ⚪ ✅, and ⚠️ in "⚠️ dựa trên thông tin ước lượng" ("⚠️ based on estimated information")) is hidden from assistive technology (`aria-hidden="true"`), so the text label is the accessible name and is not read twice. The 3:1 non-text contrast rule applies to indicators the app draws, not to emoji, whose colors come from the platform's font. Understanding a status never depends on the emoji.
- **Contrast.** Text meets 4.5:1 (SC 1.4.3); UI components and states meet 3:1.
- **Vietnamese language.** Pages use `<html lang="vi">`, and any fragment in another language carries its own `lang` (SC 3.1.1, 3.1.2). Fonts cover every Vietnamese diacritic and tone mark, and line height leaves room for stacked marks when text spacing is changed (SC 1.4.12).
- **Keyboard and screen reader.** Everything works with a keyboard alone, with no trap, a logical focus order and a visible focus (SC 2.1.1, 2.1.2, 2.4.3, 2.4.7). Controls have accessible names, roles and values (SC 4.1.2). Results such as "saved" or a validation error are announced as status messages (SC 4.1.3).
- **Focus not obscured** (SC 2.4.11, Focus Not Obscured (Minimum)). The "sắp đến hạn" ("due soon") banner and any sticky bar never cover the focused element entirely.
- **Target size** (SC 2.5.8). Targets are at least 24 by 24 CSS pixels, or spaced as the criterion allows. No action needs dragging without a single-pointer alternative (SC 2.5.7).
- **Zoom and reflow.** Text resizes to 200% (SC 1.4.4) and content reflows at 320 CSS pixels wide with no two-way scrolling (SC 1.4.10). The PWA does not lock orientation (SC 1.3.4) or block zoom.
- **Age schedule (F06).** On large screens the grid is a real `<table>`, with `<th scope="row">` for diseases and `<th scope="col">` for age milestones (SC 1.3.1). On mobile, where F06 has no grid, the list uses headings or list markup for each age group, and each item names its disease, age group and cell state in text (for example "Không khuyến cáo" ("Not recommended")). The tap-to-expand detail is a button with `aria-expanded`. A cell's state is given in text, not by fill alone.
- **Admin tables and diffs.** The admin diff and approval views use real table markup with row and column headers (SC 1.3.1). A diff shows a change with text markers such as `+` and `−`, not by color alone. Any future chart or timeline gets a text or table alternative.
- **Forms.** Every field has a label and instructions; dates say which format and precision are expected (see below); errors are identified in text with a suggestion (SC 3.3.1, 3.3.2, 3.3.3). Fields about the user (the sign-in email and the "Me" profile) use `autocomplete` tokens (SC 1.3.5). Fields about another person's profile do not use personal tokens such as `bday` or `name`, so the browser does not fill in the user's own details. Information already given in the same process is not asked for again (SC 3.3.7). Deleting a record or profile can be confirmed or undone (SC 3.3.4).
- **Imprecise dates (F04.2).** The precision of a dose date or birth date is chosen with a labeled radio group or select that states each level in text. The date fields shown change with the precision, and focus stays on the precision control. Dates can always be typed (DD/MM/YYYY, MM/YYYY or YYYY), never only picked from a calendar.
- **History quick buttons (F03.6).** The "Bạn đã từng tiêm chưa?" ("Have you ever been vaccinated?") buttons "Đã tiêm / Chưa tiêm / Không nhớ" ("Vaccinated / Not vaccinated / Don't remember") have text names, and the current choice is exposed (for example `aria-pressed`).
- **Sign-in** (SC 3.3.8). The email code field allows paste and autofill (`autocomplete="one-time-code"`). Turnstile, on the OTP form and the error report form (SAD §9), should run in a mode that does not normally show a challenge. The mode is chosen when identity hardening lands, and a barrier it causes is recorded as a known limitation. The code's expiry and the wait before a new code are security limits; they are shown in text, and once the wait is over a new code can be requested.
- **Consistent help** (SC 3.2.6). The link to the "Về dữ liệu" ("About the data") page, which holds the contact and report details (F10.2), keeps the same relative order on every page. The "Báo sai dữ liệu" ("Report a data error") button (F10.3) sits in the same position within each recommendation, disease, product and table cell.
- **Emails.** Semantic HTML with `lang="vi"`, a plain-text alternative part, readable size and contrast, descriptive link text, and no information carried only by an image or a color. The neutral subject and content rules of F08 stay as they are.

## How it is checked

- **Automated checks** in CI once `apps/web` exists: axe-core run through Playwright on the key pages and states (empty, error, dialog open, narrow mobile widths), failing the build on violations. Tool versions are the latest stable when the web app is set up. The `target-size` rule is off by default in axe-core and must be turned on. Results that axe marks as needing review go to a person.
- **Manual checks**, because automated tools find only part of the problems: keyboard-only use, 200% zoom, 320-pixel reflow and a screen reader pass on the changed screens, before each release. Screen reader testing uses real devices with Vietnamese voices installed. No screen reader or voice is named as supported until it has been tested.
- **Review.** A pull request that changes web UI or email templates is reviewed against this file.

## Known limitations

None recorded yet, since there is no web UI. Problems will be listed here as they are found, including barriers in third-party pages and widgets (the Cloudflare Access login, Turnstile).

## Reporting a barrier

Open a [Bug report](https://github.com/fugo101/vaxbook/issues/new/choose) and choose the Area "Accessibility". Describe the barrier, the page or email, and the browser, device and assistive technology you used (for example a screen reader or zoom level). The maintainer adds the `accessibility` label during triage.

The repository is public. Use made-up data and never paste a real vaccination record or other personal data.

Reports are handled on a best-effort basis. There is no response-time commitment ([`SUPPORT.md`](SUPPORT.md)).
