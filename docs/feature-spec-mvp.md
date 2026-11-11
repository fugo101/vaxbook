# VaxBook — MVP Feature Specification

| | |
|---|---|
| **Version** | 1.2 |
| **Date** | 2026-11-11 |
| **Status** | Feature stage finalized (Q1–Q54, 2 review rounds). v1.1: adjusted per SAD (see Appendix B). v1.2: engine semantics settled (Appendix C) |
| **Related documents** | [`docs/research/2026-11-10-feature-research.md`](research/2026-11-10-feature-research.md), [`docs/sad.md`](sad.md), [`GLOSSARY.md`](../GLOSSARY.md) |
| **Vietnamese version** | [`feature-spec-mvp.vi.md`](feature-spec-mvp.vi.md) (reference copy; this English file prevails) |

> This document describes only **features** (what to do, business rules, scope). Architecture, technology and infrastructure belong to the SAD.
> Items marked **(UNVERIFIED)** are based on research that has not been confirmed against the original source.

---

## 1. Overview

### 1.1 Goals
VaxBook is a web app that helps users in Vietnam:
1. Store the vaccination history of themselves and their family members.
2. Know **what to consider getting vaccinated with now**, based on age, sex, vaccination history and the age schedule table.
3. Look up the disease catalog, the vaccine lines for each disease, and the age schedule table.

### 1.2 Positioning
- **Nonprofit**, a personal support tool, run by one individual.
- Information is **for reference and recommendation only**. The decision to vaccinate belongs to the user after consulting a doctor.
- We try to build the most accurate data possible, but it **does not replace a doctor** and **is not an authoritative data source**.
- The level of responsibility may be raised in the future if an organization steps in as a sponsor.

### 1.3 Target users
| Persona | Description | Main needs |
|---|---|---|
| **Individual** | Single middle-aged person, uses it only for themselves, often does not remember childhood vaccination history | Know what they should get at their current age, reminders for periodic booster doses |
| **Parents** | Track vaccinations for young children (0–6 years is the most dose-dense period) | Not miss milestones, know which doses are free (TCMR, Vietnam's Expanded Programme on Immunization, free at commune health stations) |
| **Caregiver** | Tracks for elderly parents, may not track for themselves | Recommendations for older adults (flu, pneumococcal, shingles…) |

### 1.4 Differences from existing solutions
Existing solutions (VNeID, the national immunization system, the VNVC app, the Long Châu app, older apps no longer updated) are described in detail in the research document. VaxBook differs in:
1. A **neutral** vaccination record that collects doses from everywhere (TCMR, VNVC, Long Châu, abroad, paper vaccination booklets).
2. Recommendations **with citations**, not tied to selling anything.
3. Handles cases of **not remembering the vaccination date, not remembering the vaccine name, not remembering childhood history**.
4. Covers adults, older adults, and women who are pregnant / planning a pregnancy.

---

## 2. Product principles

| # | Principle | Application |
|---|---|---|
| P1 | **Safe when uncertain** | When data is incomplete, never recommend a dose *earlier* than is safe, and do not recommend a dose that may already be *past the maximum age*; put it in the "Chưa rõ" ("Unclear") bucket instead of guessing |
| P2 | **Always cite sources** | Every recommendation / rule has a citation and the rule set update date |
| P3 | **Reference, not prescription** | Wording "có thể cân nhắc" ("may consider"), always with "hỏi bác sĩ" ("ask a doctor") |
| P4 | **Minimal collection** | Collect only what is needed; every demographic field can be left blank |
| P5 | **Do not discourage the user** | Do not flood the user with red warnings; group unclear items into one bucket |
| P6 | **Design for extension** | Deferred features (prices, push, sharing, profile transfer…) must be addable without a large migration |

---

## 3. Scope

### 3.1 IN the MVP

| ID | Group | Features |
|---|---|---|
| F01 | Access | Email OTP sign-in; guest mode; transfer guest data to an account; deletion; withdraw consent; change email; download my data |
| F02 | Getting started | Choose who to track; accept the disclaimer; consent to data processing |
| F03 | Profiles | Multiple profiles per account; optional "Me" profile; personal / family UI inferred automatically; consent confirmation for other adults' profiles; history status when creating a profile |
| F04 | Vaccination history | Entry by disease, optional product, combination vaccine doses, date precision, "không nhớ thời nhỏ" ("don't remember childhood"), input validation, per-disease records after asking a doctor |
| F05 | Recommendation engine | Recommendations by age, sex, history, dose intervals, catch-up, product series, periodic boosters, pregnancy |
| F06 | Age schedule table | Public version + personalized version; list layout on mobile |
| F07 | Catalog | Diseases, products (no prices), links to VNVC / Long Châu |
| F08 | Notifications | In-app notification center; reminder emails batched per account; channel settings |
| F09 | Printout | Print page for one profile's vaccination history |
| F10 | Data transparency | 4-tier disclaimer; "Về dữ liệu" ("About the data") page; "Báo sai dữ liệu" ("Report a data error") button |
| F11 | Admin | Manage the catalog & rules through changesets; diff, approve, publish, rollback; emergency suspension (suspend scheduling); error report queue |
| F12 | MCP admin | Add / edit / delete data as changesets; dry-run the recommendation engine |
| F13 | AI review | 5 expert roles; block publishing when there are high-severity errors |

Platform: **mobile-first** web, **Vietnamese only**, basic PWA (installable to the home screen).

### 3.2 NOT in the MVP
Items covered in §6 are pre-designed so they are easy to add; the remaining items (center map, stock availability, vaccination packages, native app) have no pre-design yet.
- Per-source prices & price sync (VNVC, Long Châu, …)
- Web push
- Sharing one profile with multiple accounts for joint management
- **Splitting a profile into its own account / moving a profile to another account**
- Google sign-in
- Vaccination record photos, OCR
- Sync from VNeID / the national immunization system
- Recommendations based on underlying conditions, travel, occupation
- Vaccination appointment booking, center map, stock availability, vaccination packages
- English, native app

---

## 4. Detailed specification

### F01 — Access & accounts

**F01.1 Email OTP sign-in**
- An account is identified only by **email**. Do not collect phone number, real full name, citizen ID card (CCCD), or address.
- Passwordless sign-in with an OTP code sent by email.
- Signing out clears all of the account's profile and history data temporarily stored on the device.

**F01.2 Guest mode**
- Guests can: create temporary profiles, enter history, view recommendations, the age schedule table, and the catalog.
- Guest data is **stored only in the browser**; guests do not receive reminder emails and have no notification center (banner only).
- Show clearly: "Dữ liệu chỉ lưu trên thiết bị này, có thể mất nếu xoá dữ liệu trình duyệt. Trình duyệt có thể tự xoá nếu lâu không mở hoặc khi dùng chế độ ẩn danh; người khác dùng chung thiết bị có thể xem được." ("Data is stored only on this device and may be lost if browser data is cleared. The browser may clear it automatically if it is not opened for a long time or when using incognito mode; other people sharing the device can see it.")
- Guest settings always have a **"Xoá toàn bộ dữ liệu trên thiết bị này"** ("Delete all data on this device") button (with confirmation).

**F01.3 Transfer guest data to an account**
When a guest signs up / signs in and the browser still has guest data:
1. List each guest profile (nickname, number of doses) with a checkbox, all selected by default.
2. If the account does not yet have "Lưu dữ liệu tiêm chủng" ("Save vaccination data") consent → show the consent screen (F02.3) **before** uploading; if the user does not consent, nothing is uploaded.
3. If the account already has a "Me" profile and the guest data also has a "Me" profile → ask "Hồ sơ nào là bạn?" ("Which profile is you?"); the other profile becomes a family member profile (keeping its nickname). Enforce F03.2 (at most one "Me").
4. Do not automatically merge profiles with the same nickname.
5. Show the summary "Đã chuyển N hồ sơ, M mũi tiêm" ("Transferred N profiles, M dose records"), and only then delete the copy in the browser.
- The transfer is **all or nothing** for the selected profiles; running it again does not create duplicates.
- A disclaimer acceptance ticked while a guest is kept (if it is the same version).
- For each selected profile that is **not "Me"** and belongs to a person aged 18 or older or of unknown age (including a "Me" profile just turned into a family member profile in step 3): show the F03.5 checkbox for each profile before uploading and write a consent record (account, profile). Profiles that are not ticked are deselected and stay on the device.
- Declining the transfer → while signed in, guest data is not shown; Settings has a "Dữ liệu khách trên thiết bị này" ("Guest data on this device") section with "Chuyển vào tài khoản" ("Move to account") and "Xoá" ("Delete") buttons. This hiding rule applies only to accounts that **currently have** "Lưu dữ liệu tiêm chủng" ("Save vaccination data") consent (see F01.5).

**F01.4 Data deletion**
- Delete **a single profile**: deletes all of the profile's vaccination history and state; immediately cancels all unsent reminder emails related to the profile; deletes the profile's notifications. The next batched email is regenerated from the remaining data.
- Delete **the entire account**: as above for every profile of which the account is the sole owner (MVP: all profiles), plus the email, all notifications, pending emails, and consent records.
- User data is **hard deleted**; soft delete (F11.3) applies only to the catalog and rules.
- System logs store only internal identifiers, not emails or nicknames.
- **The beta phase has no backups** (SAD, ADR-0007): deleted data is gone immediately; on the other hand, a server incident can cause data loss. The "Về dữ liệu" ("About the data") page states this clearly and recommends using "Tải dữ liệu của tôi" ("Download my data") (F01.7) regularly. Once backups exist (a pre-launch condition, before public launch), this page states how long deleted data still remains in backups.
- Both actions require re-confirmation; after deletion, data cannot be restored. The confirmation dialog has a **"Tải về trước"** ("Download first") button (F01.7).

**F01.5 Withdraw "Lưu dữ liệu tiêm chủng" ("Save vaccination data") consent**
- Withdrawing consent → deletes all profiles and vaccination history on the server (as in F01.4), **keeps the account** (only the email and settings remain).
- Before withdrawing, the user can choose to **keep a copy on the device** as guest data (F01.2); the dialog has a "Tải về trước" ("Download first") button (F01.7).
- After withdrawing consent (or when the save consent was not ticked per F02.3), the user **stays signed in but uses the app as in guest mode**: guest data on the device, including the copy just kept, is shown and used as in F01.2. Signing out does **not** delete this copy.
- To save to the account again, the user must consent again → runs the F01.3 transfer flow.
- Settings has a **"Quyền riêng tư"** ("Privacy") section that lists each consent (save data, reminder emails, per-family-member-profile consent — F03.5) with its consent date, and allows withdrawing each one. Every consent / withdrawal writes a consent record with the text version.
- Withdrawing consent for **a single profile** (F03.5) → deletes that profile on the server, like deleting a single profile (F01.4); the dialog has a "Tải về trước" ("Download first") button (F01.7).

**F01.6 Change email**
- While signed in: enter the new email → confirm with an OTP sent to the new email → send a neutral notification to the old email.
- The operator **does not recover accounts manually** (identity is hard to verify, and it is easy for someone to impersonate the user to obtain health data). State this clearly on the "Về dữ liệu" ("About the data") page and in Settings.

**F01.7 Download my data**
- Export all profiles and vaccination history to a **structured file**; also available to guests.
- The "Tải về trước" ("Download first") button appears in the delete-profile, delete-account and withdraw-consent dialogs.

### F02 — Getting started (onboarding)

1. **"Bạn muốn theo dõi lịch tiêm cho ai?"** ("Who do you want to track vaccinations for?") — multi-select: `Bản thân` ("Myself") / `Người thân (con, bố mẹ…)` ("Family members (children, parents…)").
   - The choice only determines **the next profile-creation step**; it does not create any "mode" (see F03.4).
2. **Disclaimer** — the user must tick *"Tôi hiểu thông tin chỉ để tham khảo, không thay thế tư vấn của bác sĩ"* ("I understand the information is for reference only and does not replace a doctor's advice") before viewing personal recommendations. Applies to guests too.
   - The public version of the age schedule table (F06) and the catalog (F07) **do not require the tick**; the personalized version of F06 and the recommendations page are shown only after the tick.
   - The disclaimer text has a **version number**; when the version changes, the user (including guests) must tick again before viewing recommendations.
3. **Data processing consent** (only with an account) — each item separate, not pre-ticked:
   - Save vaccination data (required to save on the account). The text states clearly that **this is health information (sensitive data)**, and for child profiles: *"Tôi là cha mẹ / người giám hộ của trẻ em mà tôi tạo hồ sơ"* ("I am the parent / guardian of the children I create profiles for").
   - Receive reminder emails (optional).
   - **Not ticking** "Lưu dữ liệu tiêm chủng" ("Save vaccination data") → continue in **guest mode** (F01.2). Withdrawing consent later: see F01.5.
4. Create the first profile (F03), including the history status question (F03.6).

### F03 — Vaccinee profiles

**F03.1 Fields**
| Field | Required | Notes |
|---|---|---|
| Nickname | Yes | E.g. "Tôi" ("Me"), "Mẹ" ("Mom"), "Bé Na" ("Baby Na"). No real full name needed |
| Date of birth | No | 4 levels: full date / month-year / year only / not entered |
| Sex | No | Male / Female / blank |
| Pregnancy flag | No | See F05.7 |
| History status | Yes | See F03.6 (the option "Không nhớ lịch sử thời nhỏ" ("I don't remember my childhood history") → F04.5) |

**F03.2 "Me" profile (self profile)**
- Optional: created only when the user picks "Bản thân" ("Myself") during onboarding or adds it later in settings.
- Each account has **at most one** profile marked "Me".
- The "Me" profile addresses the user as "bạn" ("you"); family member profiles use the nickname ("Bé Na nên tiêm…" ("Bé Na should get…")).

**F03.3 Information completeness**
- Each profile shows a "Độ đầy đủ thông tin" ("Information completeness") indicator and suggests which fields to fill in for more accurate recommendations.
- For children under 2 years old without a full date of birth: a gentle reminder "Nên nhập ngày sinh đầy đủ cho trẻ nhỏ" ("You should enter the full date of birth for young children") (not required).

**F03.4 Personal / family UI — inferred automatically, no toggle**
- **1 profile** → personal UI: goes straight to that profile's recommendations page; no profile switcher; a small link "+ Thêm người thân" ("+ Add family member").
- **≥ 2 profiles** → family UI: a "Tổng quan gia đình" ("Family overview") page with a card per person, sorted by attention level (number of overdue / due items); has a profile switcher. ⚪ "Chưa nhập" ("Not entered") items (F03.6) and items in blocks outside the 5 buckets (F05.4a, including "Chưa tới lịch tiêm" ("Not yet scheduled")) **do not count** toward the attention level.
- Deleting profiles down to 1 → automatically returns to the personal UI.
- Settings has a **"Người thân"** ("Family members") section to add / edit / delete profiles.

**F03.5 Confirmation when creating a profile for another adult**
- When creating a profile that is **not "Me"** for a person **aged 18 or older** (or of unknown age): a required checkbox, not pre-ticked — *"Tôi là người giám hộ, hoặc đã được người này đồng ý cho lưu thông tin tiêm chủng"* ("I am the guardian, or this person has consented to storing their vaccination information").
- Child profiles use the general confirmation in F02.3.
- Store **consent records per (account, profile) pair** from the MVP onward — this is also the foundation for transferring / splitting profiles later (§6).

**F03.6 History status when creating a profile** (distinguishing "not entered" from "not vaccinated")
- A **required** question when creating a profile, with three options:
  1. **"Đã có mũi tiêm, tôi sẽ nhập"** ("I have had doses, I will enter them") → the profile is in the **"Chưa nhập xong"** ("Not finished entering") state: only milestones with a due date **before the moment the user picks this option** that have no record show ⚪ "Chưa nhập — kiểm tra sổ tiêm" ("Not entered — check the vaccination record"), and do not become 🔴. Milestones with a due date after that moment are calculated normally per F05.4. The profile leaves this state when the user taps **"Tôi đã nhập xong"** ("I have finished entering"), or automatically when **no assessment** of the profile is ⚪ "Chưa nhập" ("Not entered") at either end of the birth-date range. This is checked **per milestone, not per disease**: entering dose 1 of a 3-dose series does not exit the state while dose 2 is still ⚪ "Chưa nhập" ("Not entered").
  - A disease stops being pending when none of its milestones is ⚪ "Chưa nhập" ("Not entered"): its doses are recorded, it has a history declaration (F04.2 level 5, quick button below), an active acknowledged declaration (F04.6), or it is certainly out of age (F05.11 layer 5). Milestones after a level-4 dose are not pending.
  - The engine returns the list of diseases still pending, so the server and the client decide from one field. The list is computed before suspension (F11.8) and pregnancy (F05.7) are applied, so neither keeps nor releases a disease.
  - Automatic exit is evaluated **only right after the user saves** a dose record or a disease declaration — never on passive recalculation (new rule set version, rollback, suspension, passage of time).
  - A profile without a date of birth leaves the state only through the "Tôi đã nhập xong" ("I have finished entering") button.
  - A profile in "Chưa nhập xong" ("Not finished entering") must carry the time the option was chosen; without it the input is invalid (F05.1).
  2. **"Không nhớ lịch sử thời nhỏ"** ("I don't remember my childhood history") → F04.5.
  3. **"Chưa tiêm mũi nào"** ("Not vaccinated with any dose yet") → the engine treats the person as unvaccinated and calculates normally.
- **Adult profiles past the childhood series age, with option 1 or 2** (option 3 is calculated normally and may show 🔴): for past milestones of diseases that are still within age and have no record, show ⚪ "Bạn đã từng tiêm chưa?" ("Have you ever been vaccinated?") (with option 1 it replaces the "Chưa nhập" ("Not entered") label; with option 2 it sits in the "Nên hỏi bác sĩ hoặc xét nghiệm" ("Ask a doctor or get tested") bucket of F04.5) with quick buttons **Đã tiêm / Chưa tiêm / Không nhớ** (Vaccinated / Not vaccinated / Don't remember). It becomes 🔴 only when the user picks "Chưa tiêm" ("Not vaccinated"). "Đã tiêm" ("Vaccinated") creates a level-4 dose record; "Không nhớ" ("Don't remember") creates a disease declaration "chưa rõ" ("unknown") (level 5); "Chưa tiêm" ("Not vaccinated") creates a disease declaration **"đã xác nhận chưa tiêm"** ("confirmed not vaccinated") → the engine calculates normally (may show 🔴). These can be cancelled like an F04.6 record. The two declarations are **history declarations** (F04.6): when that disease has dose records, the dose record data takes precedence; they in turn take precedence over the history status. Diseases that are certainly out of age (e.g. Rota, Hib for an adult) get no quick button and go to the "Không còn trong độ tuổi tiêm" ("No longer within the vaccination age") block (F05.11).
- This option can be changed in profile settings.

### F04 — Vaccination history

**F04.1 Entering a dose record**
| Field | Required | Notes |
|---|---|---|
| Disease | Yes | Select one or more diseases |
| Product | No | Selecting a product from the catalog → the system pre-fills the diseases from the product–disease relationship (e.g. 6in1, 5in1, MMR); the user can remove some or add more |
| Vaccination date + precision | Yes (precision) | See F04.2 |
| Dose number | No | Inferred **per disease** of that dose from chronological order (e.g. the first 6in1 dose is hepatitis B dose 2 but Hib dose 1); the user can edit it. If the dose number the user enters is greater than the number of records → treat the earlier doses as existing at level 4 (**implied doses**, F04.2) |
| Vaccination site | No | Free text or pick from (TCMR / VNVC / Long Châu / nước ngoài ("abroad") / khác ("other")) |
| Lot number | No | |
| Notes | No | |

- Allow dose records with a product **outside the catalog** (e.g. vaccinated abroad): enter a free-text name and select the corresponding diseases.
- When entering a dose for an **information-only** disease (e.g. rabies): show a notice that cannot be skipped — *"VaxBook không nhắc lịch cho bệnh này, hãy theo lịch hẹn của cơ sở y tế"* ("VaxBook does not send reminders for this disease; follow the appointment schedule of your healthcare facility"). The dose is saved to the history and shown in the "Chỉ lưu trữ" ("Record only") block (F05.4a).
- **No** vaccination record photo upload in the MVP.

**F04.2 Vaccination date precision (5 levels)**
| Level | Meaning | Handling in the recommendation engine |
|---|---|---|
| 1 | Exact date | Calculated normally |
| 2 | Remembers month/year only | Treated as **end of month** (capped at today) when calculating the next dose |
| 3 | Remembers year only | Treated as **end of year** (capped at today) when calculating the next dose |
| 4 | Does not remember when, but certain it was given | Counts as an existing dose. If it may be the latest dose, the due date of the following dose cannot be calculated → ⚪ "Cần kiểm tra sổ tiêm / hỏi bác sĩ" ("Check the vaccination record / ask a doctor"), unless the disease is certainly out of age (F05.11). For a periodic booster after a level-4 last dose, the rule's unknown-last-dose behavior applies instead (F05.12). A dated dose after it is trusted (F05.10) |
| 5 | Does not remember whether vaccinated or not | No dose record is created; create a **disease declaration = "chưa rõ" ("unknown")** for the profile (a history declaration, F04.6; DiseaseDeclaration, §5) → recommendation with a note that a doctor may order an antibody test or catch-up vaccination. It covers only milestones whose due date is **before the date the declaration was created**; later milestones are calculated normally |

Every recommendation based on level 2–5 data, or on a level-4 or implied dose anywhere in the series, is labeled **"⚠️ dựa trên thông tin ước lượng"** ("⚠️ based on estimated information").

Estimated date conventions:
- A level 2–3 dose has the date range **[period start, min(period end, today)]**. This today-capped range is used alike for ordering, for dose validation (F05.10) and for calculating the next dose.
- The converted date used when calculating the next dose = the end of that range = **min(end of month / end of year, today)**.
- When ordering doses, an estimated dose uses its range; if it overlaps another dose, use the dose number the user entered; if there is none, the result for that disease is ⚪. Dose numbers entered by the user that conflict with each other → ⚪ `NUMBERING_CONFLICT`.
- Level-4 and implied doses: a **numbered** level-4 dose sits at its number. An **unnumbered** level-4 dose counts toward the number of doses given but has no position in the chronological order; if it may be the latest dose, the next dose is ⚪ `PREVIOUS_DOSE_DATE_UNKNOWN` (subject to precedence, F05.11).
- If dose 1 is level 4 or implied and a rule is anchored on the first dose (e.g. HPV 0-2-6), the earliest dated dose stands in for the first-dose date (this can only push dates later, so it is safe per P1); the result is labeled estimated, reason `FIRST_DOSE_DATE_ASSUMED`.

**F04.3 Switching vaccine type mid-series**
- The warning "Đổi loại vaccine giữa chừng — hỏi bác sĩ" ("Vaccine type switched mid-series — ask a doctor") **only appears** when the disease has a **"same-product requirement"** rule (a disease-level Rule, entered by the admin with a citation, e.g. Rota), and only when two or more distinct **known** products (catalog or retired) appear among the disease's doses that are not allowed additional doses. An unknown or off-catalog product does not trigger it on its own. The warning is informational and does not change the bucket.
- Valid combinations (e.g. monovalent birth-dose hepatitis B + 6in1) do not trigger a warning (see "allowed additional dose", F05.2).

**F04.4 Editing / deleting dose records and input validation**
- Edit or delete individual doses (deletion requires confirmation); recommendations are recalculated immediately.
- **Block saving** when: the vaccination date is in the future; the vaccination date is definitely before the date of birth (compared using the precision of both).
- **Allow saving with a warning** when: the interval is shorter than the minimum; the dose was given after the maximum age; the dose number the user entered conflicts with chronological order. Effects on recommendations: see F05.10.

**F04.5 "Tôi không nhớ lịch sử tiêm thời nhỏ" ("I don't remember my childhood vaccination history")**
- One tap when creating a profile (or in profile settings): the milestones of every series with the attribute **stage = childhood** (a series attribute, entered by the admin) whose due date is **before the moment the option was chosen** are treated as "chưa rõ" ("unknown"). Later milestones are calculated normally — the same cut-off as option 1 of F03.6 (so a young child's future milestones are not stuck in ⚪).
- This flag is only a **default**: for any disease that already has dose records or its own disease declaration, that specific data takes precedence (F04.6). Diseases that are certainly out of age go to the "Không còn trong độ tuổi tiêm" ("No longer within the vaccination age") block instead (F05.11).
- These items are grouped into **one "Nên hỏi bác sĩ hoặc xét nghiệm" ("Ask a doctor or get tested") bucket** (e.g. hepatitis B antibody test), and are not shown as individual red "quá hạn" ("overdue") items.
- Recommendations prioritize what can be done right away at the current age (annual flu, tetanus–diphtheria booster, HPV if within the age range, shingles / pneumococcal by age, MMR & varicella before pregnancy…).

**F04.6 Per-disease records (after asking a doctor)**

Disease declarations come in **two kinds**, with different precedence:
- **History declarations** — "chưa rõ" ("unknown", F04.2 level 5) and "đã xác nhận chưa tiêm" ("confirmed not vaccinated", F03.6 quick button). They describe the vaccination history: dose records for the disease take precedence over them, and they take precedence over the profile's history status (F03.6, F04.5).
- **Acknowledged declarations** — the four records in the table below. While active they **win even when dose records exist** and move the disease to the "Đã ghi nhận" ("Recorded") block (F05.11 layer 2).

| Record | Effect |
|---|---|
| "Đã có miễn dịch (xét nghiệm / đã mắc)" ("Already immune (test / had the disease)") | Moves to the "Đã ghi nhận" ("Recorded") block; **cannot be chosen** for diseases with a periodic booster (tetanus, flu…), even when dose records exist |
| "Bác sĩ khuyên không tiêm" ("Doctor advised not to vaccinate") | Moves to the "Đã ghi nhận" ("Recorded") block |
| "Hoãn đến [ngày]" ("Postpone until [date]") | Moves to the "Đã ghi nhận" ("Recorded") block; on that date the record expires and the item **automatically returns** to the bucket the engine computes, with an in-app banner (F08.1) |
| "Tôi chọn không tiêm lúc này" ("I choose not to get vaccinated at this time") | Moves to the "Đã ghi nhận" ("Recorded") block; **asks again after 12 months** (through the notification center). It stays in the block until the user answers the re-ask; it does not return automatically |
- Recorded items do not send emails and do not count toward the attention level.
- A record can be cancelled at any time → the item returns to the bucket assigned by the recommendation engine.
- Both kinds are stored as the profile's disease declaration (DiseaseDeclaration, §5), with the kind and the date the declaration was created.

### F05 — Recommendation engine

**F05.1 Inputs**
Age (from the date of birth, with its corresponding precision), sex, vaccination history (with precision), products used, the profile's disease declarations, the pregnant / planning a pregnancy flags, the profile's history status and the time it was chosen (F03.6), and the currently published **rule set version**.
- The **next dose** is computed with the rules and TCMR labels **in effect on the calculation date** (today). A recorded dose is checked against the rule in effect on its dose date **or** the rule in effect today (F05.10).
- A scheduled disease with **no series in effect** on the calculation date → ⚪ `NO_SERIES_IN_EFFECT`; it is never silently dropped.
- **Input validation** is a separate step from the calculation; the server and the guest data import run it before calculating. The engine never coerces values: an unparseable dose date gives ⚪ `INVALID_DATE` for the diseases of that record; an invalid date of birth, or a "Chưa nhập xong" ("Not finished entering") history status without the time it was chosen, is an input error and no assessment is produced.

**F05.2 Rule types to support** (derived from the VNVC vaccination booklet and research)
| Type | Example |
|---|---|
| Standard series by age milestone | Newborn hepatitis B within 24h; 6in1 at 2-3-4 months |
| **Alternative series** for the same disease (alternative plans within one series; every plan must have the same number of doses) | 2-3-4 months or 2-4-6 months (cells marked `*` in the booklet) |
| **Series by product × starting age** (start age is required whenever a (disease, product) pair has more than one series) | Rotarix 2 doses / RotaTeq 3 doses; Gardasil 9: 9–14 years 2 doses, 15–45 years 3 doses |
| Minimum age / maximum age to start / maximum age to complete | Rota must be completed before 24 or 32 weeks depending on the product; Hib not needed after 5 years |
| Minimum interval between doses (from the previous dose, and optionally from the first dose) | MMR 2 doses ≥ 1 month apart |
| **Catch-up** ("if not yet vaccinated") by age | Hepatitis A 2 doses if not yet vaccinated. A series may set an optional **catch-up age** (`catchUpAfterAge`): when the next dose would fall after that age → ⚪ `CATCH_UP_ASK_DOCTOR` "lịch tiêm bù — hỏi bác sĩ" ("catch-up schedule — ask a doctor") instead of 🟠/🔴. Dose counts that shrink with the age at the current dose are not modelled in the MVP |
| **Periodic booster** | Influenza every year; tetanus 5–10 years; Japanese encephalitis, typhoid 3 years. Each rule states whether the booster interval is counted as "months from the previous dose" or "by season", and the behavior when the last dose is unknown — at level 4, or the series is ⚪ because of the history with no dated dose ("treat as due" or "⚪ ask a doctor"); admin enters it with a source; without a source the default is ⚪. Calculation: F05.12 |
| **Allowed additional dose** | A monovalent newborn hepatitis B dose does not change the 6in1 series |
| **Same-product requirement** (disease-level rule, with citation) | Rota: should be completed with the same product (F04.3) |
| **Before pregnancy** | MMR, varicella should be given at least 3 months before pregnancy |
| **Information-only** (not scheduled) | Rabies (by exposure / risk), cholera |

- Series in progress: **continue, do not restart** (per WHO Table 3 — see the research document).
- Parameters such as tolerance, overdue buffer… are **rule fields**; admin enters them with a source; without a source the default is 0.
- **Display constants** — the 7-day overdue grace (F05.4), the 30-day and 6-month bucket thresholds (F05.4), and the 3-month "sau sinh" ("after delivery") note (F05.7) — are constants of the engine, not medical rules; the citation requirement of P2 does not apply to them.
- **Rule set validation** (checked on every changeset): every alternative plan within one series has the same number of doses — plans with different dose counts can only be entered as separate series that differ by product or by non-overlapping start age; otherwise the admin chooses one plan (with citation) or the disease is not scheduled. Start age is present whenever a (disease, product) pair has more than one series. A warning is shown when a changeset ends the last series of a scheduled disease.

**F05.3 Recommendation by disease or by product (hybrid)**
- Series are computed **per disease**; each combination vaccine dose counts as one dose for each disease it protects against. Product X's series only determines X's own attributes (number of doses, intervals, maximum age).
- No dose given yet → recommend by **disease**, listing the age-appropriate products (for a pregnant profile, without live vaccines — F05.7). When the date of birth is a range, each end uses the start-age bracket valid at that end (F05.5).
- First dose given with product X → later doses are computed from **X's series attributes**.
- **"Safest series"** — applies when the doses of one disease used different products, or a dose has an unknown product / a product outside the catalog. The **possible products** are the catalog products, including retired (soft-deleted) ones, which stay in the rule set version with their series and are treated as unknown products (F11.3). Take each dimension across the candidate series: number of doses = **largest**; minimum interval = **longest**; minimum age = **latest**; maximum age = the **smallest L** (maximum age to complete − 1 day, at the older end of the birth-date range) among the candidates that still need the dose, from which the last allowed day is derived (F05.4b).
- **Maximum ages that disagree**: if the candidates have different maximum ages that lead to different conclusions → ⚪ Chưa rõ ("Unclear"); do not report over-age (P1). The reason depends on the doses' products:
  - some dose has an unknown product → `CHECK_PRODUCT_NAME` "Kiểm tra tên vaccine trong sổ tiêm" ("Check the vaccine name in the vaccination record");
  - every dose has a named product (catalog, retired, or a named free-text product) → `MIXED_PRODUCTS` "đã dùng nhiều loại vaccine — hỏi bác sĩ" ("several vaccine types were used — ask a doctor").
- **Off-catalog product**: when the series is complete per the safest series over the catalog products → ✅ with the estimated label and "có mũi ngoài danh mục — xác nhận với bác sĩ đã đủ mũi" ("includes an off-catalog dose — confirm with a doctor that the series is complete") (reason `PRODUCT_NOT_IN_CATALOG`).
- **Merge order** of dates: (1) within each candidate series, collapse the alternative plans (F05.4); (2) across candidate series, the earliest allowed date, the due date and the overdue date each take their **latest** value; (3) then apply the maximum-age cap (F05.4b). The "có thể theo lịch …" ("may follow the … schedule") note is kept if any candidate series has alternative plans.
- The safest-series explanation lists each candidate series with its product, or "generic" for a series with no product.

**F05.4 Result buckets**

Date definitions for each next dose:
- **Earliest allowed date** = the date that satisfies the minimum age and the minimum intervals.
- **Due date** = max(earliest allowed date, recommended milestone), then capped at the last allowed day when the dose can still be given by then, i.e. max(today, earliest allowed date) is not after it (F05.4b).
- **Overdue date** = due date + **max(buffer, 7 days)**; buffer is a rule field (default 0). The 7 days is a display constant of the engine (F05.2), matching the "Bạn đã tiêm chưa?" ("Have you been vaccinated yet?") email on day 7 after the due date (F08); a sourced buffer longer than 7 days still wins. A dose capped by the maximum age has its overdue date capped too (F05.4b). Seasonal boosters use the end of the season window instead (F05.12).

| Bucket | Condition |
|---|---|
| 🔴 Quá hạn / cần tiêm bù ("Overdue / catch-up needed") | Today ≥ overdue date **and it is certain** the person is still within the eligible age. **Never** for a dose capped by the maximum age (F05.4b), a seasonal booster, or a booster shown as due after an unknown last dose (F05.12) |
| 🟠 Đến hạn ("Due") | Today ≥ earliest allowed date, due date ≤ today + 30 days, and today < overdue date. So on the due date itself the dose is 🟠, and from due date + 7 days (with buffer ≤ 7) it is 🔴. A dose capped by the maximum age stays 🟠 until its last allowed day, with "hạn chót DD/MM" ("last day DD/MM") (F05.4b) |
| 🟡 Sắp tới ("Upcoming") | Due date in (today + 30 days, today + 6 months], **or** today < earliest allowed date and due date ≤ today + 30 days |
| ⚪ Chưa rõ ("Unclear") | Not enough data to conclude, for example: the birth-date range gives results that differ in a way that matters (F05.5); the candidate products disagree (F05.3); a dose may be invalid, unless the out-of-age block wins (F05.10); the latest dose is level 4 and the disease is not certainly out of age (F04.2); no date of birth where an age limit matters (F05.5). Which reason shows, and which states it masks, follows F05.11. Has a sub-group "Nên hỏi bác sĩ hoặc xét nghiệm" ("Ask a doctor or get tested") |
| ✅ Đã hoàn thành ("Completed") | Series complete (and the booster period not yet reached), including a series complete per the catalog products while a dose has an off-catalog product (estimated label, F05.3). A series in progress with **at least one dose given** whose next dose is **more than 6 months** away also goes here, with the label "Đang theo phác đồ — mũi tiếp dự kiến MM/YYYY" ("On series — next dose expected MM/YYYY") |

With **no dose given yet** and the due date of dose 1 more than 6 months after today, the dose goes to the "Chưa tới lịch tiêm" ("Not yet scheduled") block (F05.4a), not to ✅; it moves to 🟡 and then 🟠 as time passes.

Each next dose belongs to **exactly one** bucket in the table above or one block in F05.4a; when several apply, the precedence in F05.11 decides.

**F05.4a Blocks outside the 5 buckets** (collapsed, at the bottom of the page, **not highlighted in red**, no email, not counted toward the attention level)
| Block | Content |
|---|---|
| Chưa tới lịch tiêm ("Not yet scheduled") | No dose given yet and the due date of dose 1 is **more than 6 months** after today. Item label: "Chưa tới tuổi tiêm — dự kiến MM/YYYY" ("Not yet of vaccination age — expected MM/YYYY") when today < earliest allowed date; otherwise "Theo lịch khuyến cáo — dự kiến MM/YYYY, có thể tiêm từ MM/YYYY" ("Per the recommended schedule — expected MM/YYYY, may be given from MM/YYYY"). Pregnant profile: see F05.7 |
| Không còn trong độ tuổi tiêm — hỏi bác sĩ ("No longer within the vaccination age — ask a doctor") | Past the maximum age to start, or an incomplete series past the maximum age to complete, **or the next dose can no longer be given before the maximum age** (max(today, earliest allowed date) is after the last allowed day, F05.4b) (e.g. Rota for a 7-month-old baby; Hib after 5 years) — only when **certainly** over age, i.e. at the young end of the birth-date range (P1) |
| Chỉ lưu trữ — không nhắc lịch ("Record only — no reminders") | Doses recorded for an information-only / non-scheduled disease (e.g. rabies) |
| Đã ghi nhận ("Recorded") | Items with an active acknowledged declaration (F04.6) |
| Hoãn trong thai kỳ / Trong thai kỳ ("Postponed during pregnancy" / "During pregnancy") | See F05.7 |

The **age-based** "Không còn trong độ tuổi tiêm" ("No longer within the vaccination age") states — past the maximum age to start or to complete — are mapped to the **"Không khuyến cáo"** ("Not recommended") cell in the age schedule table (F06), so the table and the recommendations do not diverge. A state that also depends on the profile's own dose dates (the next dose no longer fits before the maximum age) has no equivalent in the table, which does not know the history.

**Alternative series** (alternative plans within one series, e.g. 2-3-4 or 2-4-6 months; all plans have the same number of doses, F05.2): each plan is computed separately. The due date — and the earliest allowed date from that same plan — follow the plan that gives the **earliest due date**; the overdue date follows the **latest** plan. Across candidate series and the maximum-age cap: merge order in F05.3. The card says "có thể theo lịch 2-3-4 hoặc 2-4-6 tháng — hỏi bác sĩ" ("may follow the 2-3-4 or 2-4-6 month schedule — ask a doctor"); do not infer the alternative plan from the intervals already given. A recorded dose is valid if it satisfies the dose rule of at least one plan (F05.10).

Each recommendation shows:
- The label **TCMR (miễn phí tại trạm y tế)** ("TCMR (free at commune health stations)") or **Dịch vụ (có phí)** ("Paid service") for **each listed product** (see F07). For TCMR vaccines that are being rolled out gradually (Rota, pneumococcal, HPV): show *"TCMR — có thể chưa triển khai tại địa phương, hỏi trạm y tế"* ("TCMR — may not yet be available locally, ask the commune health station"). Do not ask for the user's province / city.
- Citation + rule set update date.
- Standard disclaimer line (F10.1 tier 2).
- The label "⚠️ dựa trên thông tin ước lượng" ("⚠️ based on estimated information") when applicable.
- The last allowed day "hạn chót DD/MM" ("last day DD/MM") when the due date is capped (F05.4b).

**F05.4b Maximum-age cap (last allowed day)**

For a series with a maximum age to complete, the engine computes a **last allowed day** ("hạn chót") per next dose so that, whenever the earliest allowed date allows it, the due date never falls after the last day the dose can still be given and the series completed in time:
- **L** = maximum age to complete − 1 day, computed at the **older end** of the birth-date range (P1).
- **L_k** (last allowed day of dose k) = L minus the sum of the minimum intervals of the doses after dose k — the last day dose k can be given and still leave room to complete the series. With no dose given yet, also take the minimum with (maximum age to start − 1 day).
- With several candidate series (unknown product, mixed products, off-catalog product), use the **smallest L** among the candidates that still need dose k. The cap is applied **after** the merge of F05.3.

Then, for the next dose k, with **F** = max(today, earliest allowed date) (the first day from today on which the dose may be given):
| Case | Result |
|---|---|
| F ≤ L_k | Due date and overdue date = min(current value, L_k), whatever the source of the buffer. If this lowers either date, the dose is **capped**: reason `DUE_CAPPED_BY_MAX_AGE`, **never 🔴**, shown 🟠 until L_k with "hạn chót DD/MM" ("last day DD/MM"); the reminder emails before the due date (7 days, 1 day) use the capped due date (F08). Otherwise the dates are unchanged and the bucket follows F05.4 |
| L_k < F ≤ L | 🟠 until L, with "có thể không kịp đủ phác đồ — hỏi bác sĩ" ("may not be able to complete the series — ask a doctor") (reason `SERIES_MAY_NOT_COMPLETE`, informational: doses are still given up to the maximum age). The chain of remaining doses is computed from F, at the young end of the birth-date range |
| F > L | "Không còn trong độ tuổi tiêm" ("No longer within the vaccination age") block when this is certain at the young end of the birth-date range; otherwise ⚪ with "Nhập ngày sinh đầy đủ" ("Enter the full date of birth") (reason `BIRTH_DATE_IMPRECISE`) |

**F05.5 Incomplete date of birth**
- **Month/year or year only** → age is a *range*; the engine evaluates both ends (the later end is capped at today). If the rule gives the same result at both ends of the range → recommend normally.
  - Differences that only change **how far away** a milestone is — between the "Chưa tới lịch tiêm" ("Not yet scheduled") block, 🟡 and ✅ "Đang theo phác đồ" ("On series") — are **not** different results: show the result of the later end with the estimate label.
  - Any difference involving 🟠, 🔴, out of age, a maximum-age cap or dose validity → ⚪ Chưa rõ ("Unclear"), with "Nhập tháng sinh để gợi ý chính xác hơn" ("Enter the birth month for a more accurate recommendation").
  - Dates driven by a minimum age or interval come from the **later** end (never remind before the minimum age); dates capped by a maximum age come from the **older** end (F05.4b).
- **Maximum age**: if the older end of the age range may already be over age but the young end is not certainly over age → ⚪ with "Nhập ngày sinh đầy đủ" ("Enter the full date of birth").
- **Start-age bracket with no dose given yet**: each end of the birth-date range uses the start-age bracket (series) valid at that end, and the two results are compared as above.
- **Age at vaccination** when both the date of birth and the vaccination date are ranges: if the two ends of the range give different series (e.g. Gardasil 9: 14 vs 15 years) → use the series with **more doses** and attach the estimate label. This rule applies only when doses exist (it is about the age at a recorded dose).
- **No date of birth** → do not compute age-based recommendations for a series with no dose given. For an in-progress series, only recommended milestones anchored on the birth date are replaced; milestones anchored on the first dose or the previous dose are computed normally, as are periodic boosters. Show the age schedule table so the user can compare on their own.
  - A birth-anchored milestone of dose k: due date = max(earliest allowed date, previous dose + (milestone of dose k − milestone of dose k−1)), i.e. the recommended gap between milestones is kept (e.g. measles dose 2 = dose 1 + 9 months, not + 1 month); labeled estimated.
  - Age-based dose validation (minimum age, maximum age to start, maximum age to complete) is skipped; interval checks are kept.
  - The next dose is ⚪ (reason `NO_BIRTH_DATE`) if it has a minimum age that is not implied by dose 1's minimum age plus the minimum intervals, or if it has a maximum age to complete. The maximum age to start applies only to dose 1.
  - Rule set validation can check that a dose's minimum age ≤ dose 1's minimum age + the sum of the minimum intervals in between.

**F05.6 Sex left blank**
- Still show sex-dependent recommendations, with a note (e.g. "áp dụng cho nữ" ("applies to females")).
- The pregnancy flag can be turned on when sex is Female or blank; **hidden when sex is Male**.

**F05.7 Pregnant / planning a pregnancy flags**
- **Pregnant**: enter the estimated delivery date, within (today, today + 10 months]; the flag turns off automatically after the estimated delivery date.
- **Planning a pregnancy**: remind the user to reconfirm after 6 months — **only via the in-app notification center**, not via email; if the user has not reconfirmed, the flag stays unchanged and the reminder keeps showing.
- Products have a **live vaccine** attribute (admin enters it with a source). When "Đang mang thai" ("Pregnant") is on:
  - Live vaccine products are removed from the products listed for an item. If no product remains and the item is in 🔴/🟠/🟡, it moves into the block **"Hoãn trong thai kỳ — hỏi bác sĩ"** ("Postponed during pregnancy — ask a doctor") and **no reminder email is sent** (F05.11 layer 8). If non-live products remain, the item stays in its bucket listing only those.
  - A live-vaccine item in the "Chưa tới lịch tiêm" ("Not yet scheduled") block or in ✅ "Đang theo phác đồ" ("On series") whose expected month is on or before the estimated delivery date shows "sau sinh — hỏi bác sĩ" ("after delivery — ask a doctor") instead of the month.
  - If a live vaccine was recorded in the 3 months before the flag was turned on → show "hỏi bác sĩ" ("ask a doctor") on that disease (reason `LIVE_VACCINE_BEFORE_PREGNANCY`).
  - When the flag turns off automatically, these items return to the normal buckets with the note "sau sinh — hỏi bác sĩ" ("after delivery — ask a doctor"). The note lasts **3 months after the estimated delivery date**, or until a dose for that disease is recorded; the 3 months is a display constant (F05.2), not a medical value.
- Vaccines commonly considered during pregnancy (tetanus, pertussis, influenza, RSV) are shown in a separate block **"Trong thai kỳ — hỏi bác sĩ về thời điểm"** ("During pregnancy — ask a doctor about timing"): not placed in the 5 buckets, no due date, no email; the TCMR label follows the product's label (F05.11 layer 4).
- The "before pregnancy" rule (MMR, varicella ≥ 3 months) applies when the "Dự định mang thai" ("Planning a pregnancy") flag is on. It only adds the note "nên tiêm xong ít nhất 3 tháng trước khi mang thai" ("should be completed at least 3 months before pregnancy") to the disease's normal assessment; it never creates 🔴 by itself. Emails never mention pregnancy or planning a pregnancy (F08).
- **Do not** give specific gestational-week milestones until there is a verified official source.

**F05.8 Scope of scheduled diseases**
- **TCMR** per TT 10/2024/TT-BYT (Circular 10/2024/TT-BYT) and TT 13/2026/TT-BYT (Circular 13/2026/TT-BYT): the 11 diseases of TT 10/2024 plus **pneumococcal** and **HPV**. The item "other diseases as prescribed by the Minister" is not scheduled.
- Pneumococcal, HPV, Rota are scheduled with **both TCMR and paid-service products**.
- Common paid-service vaccines: influenza, HPV, pneumococcal, Rota, MMR, varicella, hepatitis A, meningococcal, dengue (Qdenga), shingles (Shingrix).
- **RSV**: information-only during pregnancy (F05.7), not scheduled.
- Other diseases / vaccines remain in the catalog but are **not scheduled** → "Tham khảo bác sĩ" ("Consult a doctor").

**F05.9 Recalculation**
- Recommendations are recalculated when: the profile or history changes; a new rule set version is published or rolled back; over time (age increases, due milestones arrive).

**F05.10 Invalid doses**
- **Too early**: a **level 1** dose that violates the minimum age, the minimum interval from the previous dose or the minimum interval from the first dose → is **not** counted as completing that dose; the disease is placed in ⚪ with "Mũi này có thể sớm hơn khuyến cáo — hỏi bác sĩ có cần tiêm lại không" ("This dose may have been given earlier than recommended — ask a doctor whether it needs to be repeated").
- **After a maximum age**: dose 1 certainly given after the maximum age to start, or any dose certainly given after the maximum age to complete → ⚪ with "mũi này tiêm sau độ tuổi khuyến cáo — hỏi bác sĩ" ("this dose was given after the recommended age — ask a doctor") (reason `DOSE_AFTER_MAX_AGE`); that dose is not counted.
- **Level 2–3** doses are checked at both ends of their today-capped date range [period start, min(period end, today)] (F04.2); a violation at only one end → ⚪ (reason `DOSE_POSSIBLY_INVALID`).
- **Several candidate series** (F05.3): validation runs per candidate. `DOSE_AFTER_MAX_AGE` only when every candidate agrees; otherwise ⚪ `CHECK_PRODUCT_NAME` (some product unknown) or `MIXED_PRODUCTS` (all products named). When the candidates disagree, the engine never concludes "no dose given" or "out of age" from a dose not being counted.
- **Dose after a level-4 or implied dose**: the minimum interval to the undated predecessor cannot be checked, so it is waived — the dose is trusted (counted), the result is labeled estimated with "không kiểm tra được khoảng cách — đối chiếu sổ tiêm" ("interval cannot be checked — compare with the vaccination record") (reason `INTERVAL_UNVERIFIABLE`), and the disease may reach ✅. Only that interval is waived: the minimum age, the maximum ages and the minimum interval from the first dose are always checked whenever the needed dates exist. If no placement of the undated dose(s) satisfies every constraint (given the dated neighbours and the birth-date range) → ⚪ (reason `DOSE_INVALID`). The minimum interval of dose j is never checked against dose j−2.
- **Alternative plans**: a recorded dose is valid if it satisfies the dose rule of at least one alternative plan; the plan is never inferred from it.
- **Rule versions**: a recorded dose is valid if it satisfies the rule in effect on its dose date **or** the rule in effect today; the next dose uses the rules in effect today (F05.1).
- **Precedence**: if the person is certainly out of age and the series would still be incomplete even if every questioned dose were counted, the "Không còn trong độ tuổi tiêm" ("No longer within the vaccination age") block wins over these ⚪ results; if counting the questioned dose would complete the series, the ⚪ is kept (P1) (F05.11 layer 5).
- **Tolerance** (if any) is a rule field; admin enters it with a source; default 0. It is used **only when validating doses already given**, applies to the minimum age, the minimum interval and the minimum interval from the first dose, **never** to the maximum ages, and is never used to compute the earliest allowed date or the due date (P1). It is checked at both ends of the (today-capped) date range.

**F05.11 Precedence**

When several states apply to one next dose, the first layer that applies (highest first) decides the bucket or block. Reasons from the layers it masks are kept with the assessment.
1. Information-only disease → "Chỉ lưu trữ" ("Record only") block.
2. Active acknowledged declaration (F04.6: already immune, doctor advised against, postponed until, chose not now) → "Đã ghi nhận" ("Recorded") block.
3. Suspension of the disease, series or rule (F11.8) → ⚪ "Tạm ngưng tính lịch — tham khảo bác sĩ" ("Scheduling suspended — consult a doctor").
4. Pregnant and the disease is on the "considered during pregnancy" list → "Trong thai kỳ" ("During pregnancy") block (F05.7).
5. **Certainly out of age** (F05.4a, judged at the young end of the birth-date range; for a level-4 previous dose: today ≥ maximum age to complete, regardless of earlier dose dates) → "Không còn trong độ tuổi tiêm" ("No longer within the vaccination age") block. This wins over the history-status defaults (option 1 including the adult quick button, option 2), over a level-4 previous dose, and over dose validity when the series would still be incomplete even if every questioned dose were counted (if counting the questioned dose would complete the series, keep ⚪ — P1). Rationale: F04.5 prioritizes what can be done now, and asking a 45-year-old about Rota or Hib is noise (P5). The block is collapsed and never red.
6. History-status defaults: ⚪ "Chưa nhập" ("Not entered") (F03.6 option 1, including the adult quick button "Bạn đã từng tiêm chưa?" ("Have you ever been vaccinated?")), ⚪ "Nên hỏi bác sĩ hoặc xét nghiệm" ("Ask a doctor or get tested") (option 2, F04.5).
7. Data ⚪: history declaration "chưa rõ" ("unknown", level 5), dose validity (F05.10), level-4 previous dose (F04.2), disagreement between the ends of an imprecise date of birth (F05.5), product uncertainty (F05.3), no date of birth (F05.5).
8. The computed bucket (F05.4, F05.4b) or the "Chưa tới lịch tiêm" ("Not yet scheduled") block. Then, for a pregnant profile, live-vaccine items in 🔴/🟠/🟡 move to the "Hoãn trong thai kỳ" ("Postponed during pregnancy") block (F05.7).

Note: layer 5 above layer 6 reverses the earlier order in which "Chưa nhập" ("Not entered") came before the maximum-age check.

**F05.12 Periodic boosters**
- A periodic booster starts after the primary series is complete; its dose numbers continue those of the series.
- A disease that only has a periodic booster (e.g. adult influenza) is modelled as a one-dose series plus a booster.
- **"Every N months"** boosters: earliest allowed date = last dose + minimum interval; due date = last dose + N months; overdue date = due date + max(buffer, 7 days) (F05.4).
- **"By season"** boosters: due date = start of the season window; overdue date = end of the window (no 7-day grace). If the window passes without a dose, the next dose rolls to the next season's window (🟡 / ✅ / block); it is **never 🔴**.
- **Unknown last dose** — the last dose is at level 4, or the primary series is ⚪ because of the history (option 2, option 1 "Chưa nhập" ("Not entered"), level 5) with no dated dose: follow the rule's behavior for an unknown last dose (F05.2). "Treat as due" → 🟠 with the estimate label, never 🔴 and no "Bạn đã tiêm chưa?" ("Have you been vaccinated yet?") email; "ask a doctor" → ⚪. Without a source the default is ⚪.
- "Đã có miễn dịch" ("Already immune") cannot be chosen for a disease with a periodic booster (F04.6).

### F06 — Age schedule table

- **Public version** (guests can view it, good for SEO): rows = diseases (with sub-rows per product line, e.g. Japanese encephalitis 3 types, HPV Gardasil / Gardasil 9, hepatitis A / A+B); columns = age milestones (0–18 months by month; 2–18 years by year; groups 19–26, 27–45, 46–64, ≥ 65); cells have 3 states: **Lịch tiêm chuẩn / Lịch tiêm khuyến cáo (tiêm bù) / Không khuyến cáo** ("Standard schedule / Recommended schedule (catch-up) / Not recommended"), with the number of doses.
- **Personalized version**: highlight the profile's current age column and mark the doses already given.
- **Mobile**: no grid; show a list by age group, tap to see details. The full grid only on large screens.
- The table is generated from the **same rule set** that the recommendation engine uses (single source of truth). Its "Không khuyến cáo" ("Not recommended") cells match the "Không còn trong độ tuổi tiêm" ("No longer within the vaccination age") block only for **age-based** states; a state that depends on a profile's dose dates (the next dose no longer fits before the maximum age, F05.4b) has no cell equivalent (F05.4a).
- The top of the table (both the public and the personalized version) has the standard disclaimer line (F10.1 tier 2). The public version does not require the disclaimer tick (F02.2).

### F07 — Disease & vaccine catalog

- **Disease**: name, short description, the products that protect against it, source. Show "Có sản phẩm TCMR" ("TCMR product available") if at least one product has a TCMR label still in effect.
- **Product**: name, manufacturer, country of manufacture, diseases it protects against (many-to-many relationship), series by starting age, live vaccine attribute, source (package insert if available).
- **TCMR labels** are attached per **product × target group** (age from–to, sex, birth year if any), with an effective date from–to and a scope note — because the same disease can have both TCMR and paid-service products.
- Each disease / product page has the standard disclaimer line (F10.1 tier 2).
- **"Xem tại VNVC / Long Châu" ("View at VNVC / Long Châu") link**: admin enters the product page URL manually. Do not collect data from their sites.
- **No prices** in the MVP.

### F08 — Notifications

- **In-app notification center** (bell icon) + **"sắp đến hạn" ("due soon") banner** on the recommendations page.
- **Reminder email**: sent **7 days** and **1 day** before the due date (defined in F05.4; for a dose capped by the maximum age, the capped due date, F05.4b). Plus **one single** "Bạn đã tiêm chưa?" ("Have you been vaccinated yet?") on the **7th day after the due date** if there is still no dose; not repeated. With a buffer of 7 days or less this is the overdue date (due date + max(buffer, 7 days), F05.4). Items in the blocks outside the 5 buckets (F05.4a, including "Chưa tới lịch tiêm" ("Not yet scheduled")) and ⚪ items do not get email; a booster shown 🟠 as "treat as due" after an unknown last dose gets no "Bạn đã tiêm chưa?" ("Have you been vaccinated yet?") email (F05.12).
  - **Grouped per account**: each account receives **at most one** email per day, covering every dose of every profile whose due date is exactly 7 days or 1 day after the send date, or that is **exactly 7 days past the due date** (e.g. "Sắp đến hạn: Bé Na – Rota mũi 2; Mẹ – Cúm mùa" ("Due soon: Bé Na – Rota dose 2; Mom – seasonal flu")). Each (dose, due date, milestone) is sent only once.
  - Content is **recomputed at send time** using the current data and rule version: if the dose has been recorded, the profile has been deleted, reminders have been turned off for the profile, or the due date has changed, nothing is sent based on the old due date.
  - Due dates based on estimated dates are marked "(ước lượng)" ("(estimated)").
  - Only sent when the user has consented to receive reminder emails (F02.3).
  - **Subject and preview line are always neutral** (e.g. "VaxBook: bạn có lịch nhắc mới" ("VaxBook: you have a new reminder")): no nickname, disease name, or vaccine name.
  - The email body contains no date of birth, lot number, notes, or vaccination place; it **never** mentions pregnancy or planning a pregnancy; the email ends with the standard disclaimer line.
  - Content is reminders only: **no** advertising, no promotions.
- **Notification settings**: turn email on / off (this is the same as recording or withdrawing consent for "Nhận email nhắc lịch" ("Receive reminder emails"); turning it back on requires ticking consent again); turn reminders on / off **per profile**; the "Thông báo đẩy" ("Push notifications") item shows a **"Sắp có"** ("Coming soon") label.
- Guests: in-app banner only, no email, no bell.

**F08.1 Notification events**
| Event | In app | Email | Guest |
|---|---|---|---|
| Dose due in 7 days / 1 day | Yes | Yes (grouped) | Banner only |
| 7 days past the due date and the dose is not yet recorded ("Bạn đã tiêm chưa?" ("Have you been vaccinated yet?"), once) | Yes | Yes (grouped) | Banner only |
| Reconfirm "Dự định mang thai" ("Planning a pregnancy") | Yes | No | Banner only |
| Re-ask "Tôi chọn không tiêm lúc này" ("I choose not to get vaccinated at this time") after 12 months (F04.6) | Yes | No | Banner only |
| "Hoãn đến [ngày]" ("Postpone until [date]") date reached (F04.6) | Yes | No | Banner only |
| A rule / disease under "Tạm ngưng tính lịch" ("Scheduling suspended") (F11.8) affects the profile | Yes (grouped with the row below) | No | Banner |
| The profile's recommendation changes bucket or due date because of a new rule version / rollback | One grouped notification, with a link to the change notes (F10.2) | No | Banner on the next open |

### F09 — Vaccination history printout

- A print page (print from the browser / save as PDF) of the full history of **one profile**: disease, product, date (with precision), dose number, vaccination place, lot number.
- Footer: *"Do người dùng tự khai, không phải giấy xác nhận y tế"* ("Self-reported by the user, not a medical certificate") + disclaimer.

### F10 — Data transparency & disclaimer

**F10.1 4-tier disclaimer**
1. Onboarding: mandatory tick, versioned (F02.2).
2. **Standard disclaimer line** "Tham khảo · Nguồn: … · Hỏi bác sĩ trước khi tiêm" ("For reference · Source: … · Ask a doctor before vaccination") — a single template, shown **everywhere a schedule or recommendation is displayed**: recommendation cards, the "sắp đến hạn" ("due soon") banner, notification center items, reminder emails, the top of the age schedule table (both versions), disease / product pages.
3. Printout footer (F09).
4. "Về dữ liệu" ("About the data") page (F10.2).

**F10.2 "Về dữ liệu" ("About the data") page**
- Data sources: VNVC vaccination booklet (printed edition, publication year unknown); TT 10/2024/TT-BYT; TT 13/2026/TT-BYT; product package inserts.
- **Source priority order**: Bộ Y tế (Ministry of Health) documents > package inserts > booklets / websites of vaccination chains. Where they differ, note it clearly.
- Update date and version number of the current rule set, with plain-language **change notes** for each version (F11.5).
- State clearly: the data is reviewed by AI and the operator, **not yet validated by a doctor** (update when it is).
- Beta period: data is **not backed up** and may be lost if the server fails; users should download their data periodically. Once backups exist: how long deleted data remains in backups (F01.4).
- The operator **does not manually recover accounts** when a user loses access to their email (F01.6).
- Nonprofit product, run by an individual; how to contact / report errors.

**F10.3 "Báo sai dữ liệu" ("Report a data error")**
- A button on every recommendation, every disease, every product, every cell of the age schedule table. Guests can also submit.
- Reports **store only**: error type (wrong dose / wrong interval / wrong age / wrong TCMR label / wrong source / potentially harmful / other), description, reference source (optional), ID of the reported object (rule / disease / product / table cell), rule set version number at the time of reporting.
- **Do not** store the account, profile, or nickname. Directly above the description field: "Vui lòng không ghi tên, ngày sinh hay thông tin sức khoẻ của người thật." ("Please do not enter the names, dates of birth, or health information of real people.")
- Reports go into the admin queue (F11.6); they can be turned into a changeset via MCP (F12) after screening.

### F11 — Admin

- **F11.1** Only admin (the project owner) has access. The admin pages only manage the catalog, rules, and the error report queue; there are **no** screens to view, search, or edit users' accounts, profiles, or vaccination histories (aggregate counts only).
- **F11.2** Manage: diseases, products, product–disease relationships, series, rules (all types in F05.2), TCMR labels (per product × target group, with effective dates and scope note), citations, provider links.
- **F11.3 Changeset**: every medical / catalog data change goes through a **changeset** (a bundle of add / edit / delete). Delete is **soft delete**.
  - Soft delete or rollback **does not remove** an entity referenced by a user's dose record: the dose record still shows the product name with "Không còn trong danh mục" ("No longer in the catalog"), is treated as an unknown product (F05.3), and that product can no longer be selected for new doses.
- **F11.3a Changeset lifecycle**: Draft → Pending review → Approved → Published; plus two states, Rejected and Stale.
  - MCP only edits changesets in **Draft** state. Submitting for review locks it into a **numbered revision**; any later edit creates a new revision, returns it to Draft, and invalidates the previous review + test results.
  - Admin approves exactly one revision number.
  - Each changeset records its **base version**. When the current version has changed → the changeset becomes **Stale**, must be updated to the new version (with a warning if the same field was edited) and reviewed + tested again.
- **F11.4 Approval**: view the diff against the current version, view the AI review results (F13) and the test case run results; approve or reject. Citations to sources marked "(CHƯA KIỂM CHỨNG)" ("(UNVERIFIED)") or to superseded sources are flagged with a warning on the approval page.
- **F11.5 Publish & rollback**
  - Publish conditions (all 4 required): (1) the latest revision has review results from **all 5 roles** (F13); (2) no high-severity finding remains in Open state; (3) no failing test cases remain; (4) every scheduling rule has at least one citation that gives a **specific location** (article / clause / page).
  - Publishing creates a **new rule set version**, with plain-language **change notes** (shown in F10.2).
  - **Rollback** always creates a new version whose content equals the target version, requires a reason, and does not require re-review (the target version has already passed the gate); all open changesets become Stale. Rollback **keeps** every suspension (F11.8) currently in effect, even when the target version was created before the suspension; a suspension is lifted only via a changeset per F11.8.
- **F11.6 Error report queue**: list of reports from F10.3; states New → **Screened** (admin has read it and redacted personal information, if any) → In progress → Resolved / Dismissed (reason required); linked to changesets. Reports of type "potentially harmful" go to the top of the queue.
- **F11.7 Audit log** (append-only, no edit / delete): every change with who proposed it (admin or AI, and which role), timestamp, citation; approve / reject (with revision number); publish; rollback (with reason); suspend / restore scheduling (with reason); finding dismissal (with reason); every MCP write command.
- **F11.8 Suspension (suspend scheduling; emergency safety valve)**
  - Admin suspends scheduling for **a series (Series), a special rule (Rule), or a disease** when a dangerous error is found in the published version. Individual dose rules are not suspended on their own (to avoid a "half-correct" series).
  - Suspending a rule → only the items produced by that rule; suspending a disease → all items of that disease. These items show in ⚪ with the label **"Tạm ngưng tính lịch — tham khảo bác sĩ"** ("Scheduling suspended — consult a doctor"), unless a higher layer of F05.11 applies (layer 1 information-only disease → "Chỉ lưu trữ" ("Record only"); layer 2 active acknowledged declaration → "Đã ghi nhận" ("Recorded")). The corresponding cell in the F06 table shows "Tạm ngưng — hỏi bác sĩ" ("Suspended — ask a doctor"). **Immediately cancel** related reminder emails.
  - The action creates a new version, **requires a reason**, and is **exempt** from the F11.5 publish conditions (exempt from AI review and testing) because it only makes recommendations more cautious (P1).
  - Restoring scheduling must go through a normal changeset (meeting the F11.5 conditions).
  - Only admin can do this, in the admin pages; MCP does not have this permission.

### F12 — MCP server for admin

- **Admin only**, authenticated with a private key.
- Scope: **catalog and rule set only**. **No** access to user data.
- Tools:
  - Read the catalog, rules, versions, changesets, and error reports in **Screened** state (F11.6).
  - **Create / update changesets** (Draft state only) containing add / edit / delete operations.
  - **Add** review results to a changeset (cannot edit / delete existing results).
  - Run the recommendation engine on **simulated profiles** (test cases).
  - Compare rules against source documents.
- Changing a test case's **expected result** is a separate operation in the changeset, labeled "Đổi kỳ vọng kiểm thử" ("Test expectation changed") in the diff.
- **Not allowed**: publish, approve, rollback, suspend scheduling, change finding status. Only admin performs these actions, in the admin pages.

### F13 — Multi-role AI review

| Role | Checks |
|---|---|
| Pediatrician / preventive medicine doctor | Clinical logic, catch-up rules |
| Pharmacist | Cross-check against package inserts: doses, minimum / maximum age, dose intervals |
| Health law / regulatory | Cross-check against TT 10/2024, TT 13/2026; TCMR labels |
| Data testing (QA) | Consistency; generate test cases (e.g. "7-month-old baby not vaccinated against Rota → past the maximum age to start") |
| Content editor | Neutral tone, not phrased as a medical order, easy to understand |

- **When**: mandatory before every publish; on admin request; monthly periodic review of new documents.
- Output: a list of findings with **severity** (high / medium / low) and **citation**. Findings without a source are shown as "chưa có căn cứ" ("unsupported").
- Any **high**-severity finding in Open state → blocks publishing (F11.5).
- AI review **does not replace** qualified professionals (see §8).

**F13.1 Finding lifecycle**
- States: **Open** → **Fixed** (linked to the changeset revision) or **Dismissed** (reason required; for high severity, a counter-source is required).
- Only admin changes finding status, in the admin pages.

**F13.2 Minimum mandatory test cases** (QA generates and maintains them; every changeset must run through them)
- Newborn hepatitis B + 6in1 × 3 → ✅, no product-switch warning.
- Rota with unknown product → safest series (F05.3).
- Rotarix dose 1 + RotaTeq dose 2 → not completed, with product-switch warning.
- 7-month-old baby not vaccinated against Rota → no longer within the age to start.
- Dose given before the minimum age / before the minimum interval has passed → not counted (F05.10).
- Overlapping estimated vaccination dates → ⚪ (F04.2).
- Gardasil 9 started at estimated age 14–15 → 3-dose series, estimate label (F05.5).
- Pregnant + live vaccine due → "Hoãn trong thai kỳ" ("Postponed during pregnancy") block, no email (F05.7).
- Profile "Chưa nhập xong" ("Not finished entering"), 6-month-old baby, 0 records → every due milestone before the moment the option was chosen is ⚪ "Chưa nhập" ("Not entered"), no 🔴, except diseases certainly out of age, which go to the "Không còn trong độ tuổi tiêm" ("No longer within the vaccination age") block (F03.6, F05.11).
- Profile "Chưa nhập xong" ("Not finished entering"), due milestone **after** the moment the option was chosen → computed normally (🟠 / 🔴, with email) (F03.6).
- 5-week-old baby, 6in1 earliest at 6 weeks, due at 2 months → 🟡 (F05.4).
- Rollback to a version from before the suspension → the suspension is still kept (F11.5).
- 45-year-old adult, option 1 or 2, no records yet → quick button "Bạn đã từng tiêm chưa?" ("Have you ever been vaccinated?") for diseases still within age (e.g. hepatitis B), no 🔴 until "Chưa tiêm" ("Not vaccinated") is selected; diseases certainly out of age (e.g. Rota, Hib) go to the "Không còn trong độ tuổi tiêm" ("No longer within the vaccination age") block (F03.6, F05.11).
- 45-year-old adult, option 3 "Chưa tiêm mũi nào" ("Not vaccinated with any dose yet") → computed normally, may have 🔴 (F03.6).
- ≥ 1 dose given, next dose more than 6 months away → ✅ "Đang theo phác đồ" ("On series") (F05.4).
- "Đã có miễn dịch" ("Already immune") for tetanus → cannot be selected (F04.6).
- Buffer 0: today = due date → 🟠; today = due date + 7 days → 🔴 (F05.4).
- No dose given, first milestone more than 6 months away → "Chưa tới lịch tiêm" ("Not yet scheduled") block, no email (F05.4a).
- Series with a maximum age to complete and a first-dose-anchored milestone after it (e.g. maximum age to complete 20 years, dose 2 at first dose + 12 months, dose 1 at 19 years 7 months) → 🟠 "hạn chót" ("last day"), `DUE_CAPPED_BY_MAX_AGE`, never 🔴; the same with a month-precision date of birth uses the older end (F05.4b). These are test inputs, not medical values.
- RotaTeq dose 1 at 14 weeks, today 29 weeks → dose 2 still shown, `SERIES_MAY_NOT_COMPLETE` (F05.4b).
- Rotarix dose 1 + RotaTeq dose 2, today past Rotarix's limit but before RotaTeq's → ⚪ `MIXED_PRODUCTS` (F05.3).
- Rule set with alternative plans of different dose counts in one series → rejected by validation (F05.2).
- Rotarix dose 1 at 16 weeks 4 days → ⚪ `DOSE_AFTER_MAX_AGE` (F05.10).
- HPV dose 1 dated, dose 2 level 4, dose 3 dated with a feasible placement → ✅ estimated, `INTERVAL_UNVERIFIABLE`; with no feasible placement → ⚪ `DOSE_INVALID` (F05.10).
- No date of birth, measles dose 1 given → dose 2 due = dose 1 + 9 months, estimated (F05.5).
- Birth year only, straddling the HPV 9–14 / 15–45 years boundary, no dose → each end uses its own start-age bracket (F05.5).
- 3-year-old with one Rota dose at level 4 → "Không còn trong độ tuổi tiêm" ("No longer within the vaccination age") block (F05.11).
- "Chưa nhập xong" ("Not finished entering"), 1 of 3 pertussis doses entered → still "Chưa nhập xong" ("Not finished entering"), dose 2 stays ⚪ "Chưa nhập" ("Not entered") (F03.6).
- "Không nhớ thời nhỏ" ("Don't remember childhood") chosen for an infant; a childhood milestone due after that moment → computed normally (F04.5).
- Off-catalog product, series complete per the catalog → ✅ with `PRODUCT_NOT_IN_CATALOG` (F05.3).
- Month-precision date of birth, one end 🟡 and the other "Chưa tới lịch tiêm" ("Not yet scheduled") → not ⚪ (F05.5).
- "Đã có miễn dịch" ("Already immune") for a disease that has dose records → "Đã ghi nhận" ("Recorded") (F04.6).
- Seasonal booster window passes without a dose → next season, never 🔴 (F05.12).
- Pregnant, live and non-live products available for the same disease → only non-live products listed (F05.7).

---

## 5. Conceptual data model

> Conceptual level, for the SAD to detail; not a schema.

```
── User data ────────────────────────────────────────────────────────────────────
Account ──< ProfileMembership >── Profile
              (role, is_self,        ├──< DoseRecord ──< DoseRecordDisease >── Disease
               notify_enabled)       │     (optional product_id, free-text product name,  (dose number
                                     │      date + precision, place, lot number,            per disease)
                                     │      notes, record source = "self-entered")
                                     ├──< DiseaseDeclaration (disease, created at,
                                     │      kind = history: unknown / confirmed not vaccinated
                                     │      | acknowledged: already immune / doctor advised against vaccination /
                                     │        postponed until date / chose not to vaccinate + ask-again date;
                                     │      reason = level 5 / doesn't remember childhood / quick button / user-recorded)
                                     ├── PregnancyStatus (pregnant + estimated delivery date / planning + reconfirm date)
                                     └── (nickname, date of birth + precision, sex,
                                          history status = not finished entering / doesn't remember childhood / not vaccinated / finished entering,
                                          with the time it was selected)

── Catalog & rules (all belong to one RuleSetVersion) ────────────────────────────
RuleSetVersion ──< Disease, Product (live vaccine attribute), ProductDisease,
                   Series (disease required; product optional; start age range — required when a (disease, product)
                           pair has several series; stage; optional catch-up age; alternative plans, same dose count)
                     ──< Dose rule (recommended milestone, minimum / maximum age, minimum interval from previous /
                                    first dose, buffer, tolerance),
                   Rule (disease-level: periodic booster — every N months or by season window, behavior when the
                         last dose is unknown; pre-pregnancy; allowed additional dose; same-product requirement;
                         information-only),
                   TCMR label (product × target group, effective from–to, scope note),
                   Citation ──> SourceDocument (specific location: article / clause / page),
                   ProviderOffering ──> Provider (VNVC, Long Châu)   [MVP: URL only]
                   + change notes
Changeset (base version, status) ──< Revision ──< ChangeOperation (add / edit / soft delete)
                                                   ──< AIReviewFinding (role, severity, source, status)
                                                   ──< TestRun ──> TestCase (simulated profile, expected result)

── Other ─────────────────────────────────────────────────────────────────────────
FeedbackReport (not linked to account / profile), Notification, EmailDispatch (dose × due date × milestone sent),
ConsentRecord (type, text version, timestamp, account level or account–profile pair, consent / withdraw),
DisclaimerAcceptance (version), RuleSuspension (series / rule / disease, reason — carried into every later version, including versions created by rollback, until restored via a changeset), AuditLog (append-only)
```

Key points:
- `Profile` is an **independent entity**, not owned directly by `Account`. The relationship goes through `ProfileMembership`.
- `is_self` ("Me") and reminder settings live on the **membership**, not on `Profile` — because "Me" is a relationship between a user and a profile.
- All medical data (`DoseRecord`, `PregnancyStatus`, flags…) is attached to `profile_id`, **never** to `account_id`.
- A dose record is linked to a **disease** via `DoseRecordDisease` (required ≥ 1); the product is optional.
- `DiseaseDeclaration` has two kinds with different precedence (F04.6): a **history** declaration yields to the disease's dose records and overrides the history status; an **acknowledged** declaration wins while active, even when dose records exist. The creation date bounds which milestones a "chưa rõ" ("unknown") declaration covers (F04.2).
- A recommendation always references the `RuleSetVersion` used to compute it.
- Every catalog entity has a **stable ID that is never reused** across versions, so users' dose records can always reference it.
- Date of birth and dose dates are always stored **with their precision**.
- User data is hard deleted; catalog & rules are soft deleted.

---

## 6. Design for extension

| Future feature | Preparation in the MVP |
|---|---|
| **Pricing & price sync** | `Provider` and `ProviderOffering` already exist; later add `PriceSnapshot` (price, price type — single / pre-order / package, recorded date, source) attached to `ProviderOffering`. The price data source must be a replaceable layer (manual entry → automated source when permitted) |
| **Web push** | The notification model separates "content" from "delivery channel"; settings already have a "Thông báo đẩy — Sắp có" ("Push notifications — Coming soon") item |
| **Profile sharing** | `ProfileMembership` is many-to-many with `role` (MVP only `owner`; later add `editor`, `viewer`) |
| **Transfer a profile to another account** | Only a `ProfileMembership` change (add the new owner, remove the old owner). Medical data does not need to move because it is attached to `profile_id` |
| **Split a profile into its own account** | Create a new `Account` (via email invitation) + transfer membership as above; `is_self` is set on the new account's membership |
| **Consent on transfer / split** | `ConsentRecord` per (account, profile) pair **already exists from the MVP** (F03.5); the new account only needs to record its own consent |
| **Ownership log** | `AuditLog` records every membership change |
| **Google sign-in** | Accounts are identified by email; the sign-in method is a separate component |
| **OCR of record photos, VNeID sync** | `DoseRecord` has a "record source" field (self-entered / OCR / synced) |
| **Underlying conditions, travel, occupation** | The rule engine accepts an extensible set of "conditions" (MVP only has pregnant / planning a pregnancy) |
| **English** | Catalog / rule content is separated from the UI so it can be translated |
| **Rule set backup via git** (high priority, post-MVP) | Endpoint that exports every version / changeset / finding (no personal data); a daily GitHub Action commits to `rules-data/`; restore command (SAD §14) |

---

## 7. Seed data

1. **VNVC vaccination booklet (printed)** — AI transcribes it into structured data → AI roles cross-check → admin approves.
2. **TT 10/2024/TT-BYT** (Circular 10/2024/TT-BYT) and **TT 13/2026/TT-BYT** (Circular 13/2026/TT-BYT) — cross-check TCMR labels; add pneumococcal, HPV. The detailed PCV / HPV schedule in the appendix of TT 13/2026 **(UNVERIFIED)**.
3. **Package inserts** — for Qdenga, Shingrix, RSV (not in the booklet) and each product's series.
4. **Rabies, cholera** — information display only, no scheduling.
5. When the VNVC booklet differs from Bộ Y tế (Ministry of Health) documents → **the Ministry of Health document takes precedence** and the difference is noted.

---

## 8. Risks & mitigations

| Risk | Level | Mitigation |
|---|---|---|
| **Wrong medical rules** (dosage, interval, age) | Highest | Principle P1; cite sources; 5-role AI review + test cases; block publishing when there are high-severity errors; report-error button; 4-tier disclaimer; recommend a one-time doctor / pharmacist check before public launch |
| **Personal data protection** | Medium | Luật BVDLCN 91/2025 (Law 91/2025/QH15 on Personal Data Protection) and Nghị định 356/2025 (Decree 356/2025/ND-CP) apply based on data processing, not on profit motive; no exemption for individuals found yet **(UNVERIFIED)**. Mitigation: minimal collection (P4), separate consent items, deletable data, encryption, no user data given to AI/MCP |
| **Outdated data** (new circulars, new products) | Medium | Monthly AI review of new documents; show the rule set update date |
| **Users drop off** (doses spaced months / years apart) | Medium | Reminder emails grouped per account; no flood of red warnings (P5) |
| **AI review confidently wrong** | Medium | AI has no publish permission; admin approves every changeset; findings must include a source |

---

## 9. Launch

- **Closed beta** by invitation (a few dozen families we know): AI review + admin approval is enough.
- **Public**: recommend an additional one-time check of the rule set by a doctor or pharmacist. The project owner decides the timing.

---

## 10. Handoff to the SAD

SAD topics to decide (not part of this document):
- Overall architecture, front-end / back-end technology, hosting (domestic or abroad).
- Guest data storage in the browser and the mechanism to move it to an account.
- Rule engine design (rule representation, assessment, testing, versioning).
- Email sending service, reminder sending schedule.
- MCP server architecture and AI review flow.
- Security, encryption, admin / MCP authentication.

---

## Appendix A — Decision log

| # | Decision |
|---|---|
| Q1, Q27, Q43, Q44 | One account, many profiles; supports individual users; UI derived from the number of profiles; "Me" profile is optional |
| Q2 | Reference tool, with disclaimer, cites sources |
| Q3 | No history sync from VNeID / chain accounts; users enter data themselves |
| Q4, Q18, Q19 | Pricing dropped from the MVP; designed to add VNVC + Long Châu prices later |
| Q5 | Nonprofit |
| Q6 | Recommendations by age, sex, history, interval, catch-up + pregnancy flag |
| Q7, Q22 | Guest mode; email OTP sign-in only |
| Q8, Q29, Q45 | No web push; notification center + reminder emails 7 days / 1 day ahead, grouped per account, on / off per profile |
| Q9 | Mobile-first web, Vietnamese, basic PWA |
| Q10 | 5-level dose date precision, computed in the safe direction |
| Q11 | Pick the disease first, product optional, supports combination vaccine doses |
| Q12 | Hybrid recommendation by disease / product, warning on switching manufacturer |
| Q13 | 5 result buckets, TCMR / Dịch vụ ("Paid service") labels (no prices shown) |
| Q14 | Medical data approved by admin, versioned and sourced |
| Q15 | No profile sharing yet; many-to-many data model |
| Q16 | Printable vaccination history |
| Q17, Q36 | Working name: VaxBook (an app with the same name exists in South Africa; vaxbook.com is already registered by someone) |
| Q20 | Province not asked; note that TCMR may not yet be rolled out locally |
| Q21 | TCMR (11 diseases in TT 10/2024 + pneumococcal + HPV) + common paid-service vaccines are scheduled; RSV is information-only |
| Q23 | Pregnancy flag with estimated delivery date, auto-expires |
| Q24 | Move guest data to an account; delete individual profiles |
| Q25 | Operated by an individual; closed beta first |
| Q26, Q33, Q34, Q39 | MCP for admin: add / edit / delete via changeset; 5-role AI review; admin approves and publishes |
| Q28 | "Không nhớ lịch sử thời nhỏ" ("Don't remember childhood history") → bucket "Nên hỏi bác sĩ hoặc xét nghiệm" ("Ask a doctor or get tested") |
| Q30 | Catalog without prices, with VNVC / Long Châu links |
| Q31, Q40 | Seed data from the VNVC booklet; Ministry of Health documents take precedence when they differ |
| Q32 | Public + personalized age schedule table; list view on mobile |
| Q35 | Public launch: recommend an additional one-time expert check |
| Q37 | Minimal collection: email, nickname, date of birth (4 levels), sex (optional); no photos |
| Q38 | 4-tier disclaimer, "Về dữ liệu" ("About the data") page, "Báo sai dữ liệu" ("Report a data error") button |
| Q42 | Handling incomplete date of birth / sex |
| (after Q45) | Designed ahead for splitting a profile into its own account / transferring a profile to another account |
| Review 1 | 13 direct edits (due date definition, safest series, invalid doses, changeset lifecycle, email…) |
| Q46 | History status question when creating a profile; "Chưa nhập xong" ("Not finished entering") shows ⚪; adults get the quick button "Bạn đã từng tiêm chưa?" ("Have you ever been vaccinated?") |
| Q47 | Blocks outside the 5 buckets: "Không còn trong độ tuổi tiêm" ("No longer within the vaccination age"), "Chỉ lưu trữ" ("Record only"), "Đã ghi nhận" ("Recorded"); doses > 6 months go into ✅ "Đang theo phác đồ" ("On series") |
| Q48 | "Bạn đã tiêm chưa?" ("Have you been vaccinated yet?") email, sent once on day 7 after the due date |
| Q49 | Per-disease record: immune / doctor advised against vaccination / postponed / chose not to vaccinate |
| Q50 | No consent to storage → guest mode; withdraw consent → delete server data, keep the account; "Quyền riêng tư" ("Privacy") section |
| Q51 | "Tạm ngưng tính lịch" ("Suspend scheduling") safety valve for admin |
| Q52 | Change email, "Tải dữ liệu của tôi" ("Download my data"), no manual restore |
| Q53 | Per-profile consent confirmation for other adults; one shared confirmation for children |
| Regression review | 7 edits (the 🟠/🟡 gap, rollback keeps suspensions, timestamp of "Chưa nhập xong" ("Not finished entering"), the "đã xác nhận chưa tiêm" ("confirmed not vaccinated") status…) |
| Q54 | Quick button "Bạn đã từng tiêm chưa?" ("Have you ever been vaccinated?") for adults on options 1 and 2; option 3 is computed normally |

---

## Appendix B — v1.1 changes (2026-11-10, following the SAD)

| # | Change | Source |
|---|---|---|
| B1 | Terminology: `ProfileDiseaseStatus` / "trạng thái bệnh" ("disease status") → `DiseaseDeclaration` / "khai báo theo bệnh" ("disease declaration"); add Assessment, Bucket, Block, Next dose | GLOSSARY.md, SAD Q7, Q15 |
| B2 | F01.4, F10.2: the beta has no backups; state this clearly on the "Về dữ liệu" ("About the data") page | SAD Q22, Q31, ADR-0007 |
| B3 | F11.8: suspension targets are Series, Rule or Disease | SAD Q8 |
| B4 | §6: add "Rule set backup via git" (high priority, post-MVP) | SAD Q30 |
| — | The 13 semantic gaps in the recommendation engine found by the prototype (SAD §5.6 B1–B13) are **settled in v1.2** (Appendix C) | SAD §5.6 |

---

## Appendix C — v1.2 changes (2026-11-11, engine semantics)

Decisions EQ1–EQ27 from the engine-semantics decision session (owner-approved). Numbered EQ to avoid clashing with Q1–Q54 (Appendix A) and B1–B4 (Appendix B). Where a later item refines an earlier one, the later item wins.

| # | Decision (summary) | Settles SAD §5.6 | Spec sections changed |
|---|---|---|---|
| EQ1 | Overdue date = due date + max(buffer, 7 days); 7 days is an engine display constant; 🟠 on the due date, 🔴 from due + 7; a capped dose is never 🔴 | B1 | F05.2, F05.4, F08 |
| EQ2 | No dose yet and dose 1 due more than 6 months away → new block "Chưa tới lịch tiêm" ("Not yet scheduled"): collapsed, not red, no email, not in the attention level | B2 | F03.4, F05.4, F05.4a, F08 |
| EQ3 | Due date after the maximum age → `DUE_CAPPED_BY_MAX_AGE` (superseded by EQ15) | B3a | F05.4b |
| EQ4 | Series can no longer be completed → `SERIES_MAY_NOT_COMPLETE`, informational (superseded by EQ15) | B3b | F05.4b |
| EQ5 | All alternative plans in one series must have the same dose count (rule set validation); otherwise separate series by product / non-overlapping start age, one plan, or not scheduled | B4 | F05.2, F05.4a |
| EQ6 | Merge order: alternative plans within each series → latest dates across candidate series → maximum-age cap | B5 | F05.3, F05.4a |
| EQ7 | Doses given after a maximum age → ⚪ `DOSE_AFTER_MAX_AGE`, not counted; one-end violation → `DOSE_POSSIBLY_INVALID`; validation per candidate; level 2–3 ranges capped at today for ordering, validation and next dose | B7a | F04.2, F05.10 |
| EQ8 | Dose after a level-4 / implied dose: interval to the undated predecessor waived, dose counted, estimated, `INTERVAL_UNVERIFIABLE`; never check dose j against j−2 | B7b | F04.2, F05.10 |
| EQ9 | No birth date (superseded by EQ19) | B8 | F05.5 |
| EQ10 | No dose yet: each end of the birth-date range uses its own start-age bracket; "more doses" rule only when doses exist | B9 | F05.3, F05.5 |
| EQ11 | Certainly out of age wins over the level-4 ⚪ | B10 | F04.2, F05.11 |
| EQ12 | "Chưa nhập xong" ("Not finished entering") exits by button or when no assessment is `NOT_ENTERED` (per milestone); engine returns the pending diseases | B11 | F03.6 |
| EQ13 | "Không nhớ lịch sử thời nhỏ" ("I don't remember my childhood history") covers only childhood milestones due before the option was chosen | B12 | F04.5 |
| EQ14 | Possible products = catalog incl. retired; off-catalog dose with series complete per the catalog → ✅ estimated, `PRODUCT_NOT_IN_CATALOG` | B13 | F05.3, F05.4 |
| EQ15 | Unified maximum-age cap: last allowed day L / L_k at the older end, smallest L across candidates, applied after the merge; capped dose 🟠 "hạn chót" ("last day"), never 🔴; `SERIES_MAY_NOT_COMPLETE`; out-of-age block extended; F06 sync limited to age-based states | B3, B6 | F05.3, F05.4, F05.4a, F05.4b (new), F06, F08 |
| EQ16 | Candidates disagree on maximum age: `MIXED_PRODUCTS` when every dose has a named product, `CHECK_PRODUCT_NAME` only when some product is unknown | B6 | F05.3, F05.10 |
| EQ17 | Two-end comparison: differences only in distance (block / 🟡 / ✅ on series) are not different results; min-age dates from the later end, max-age caps from the older end; later end capped at today | — (SAD §5.3 step 4) | F05.5 |
| EQ18 | Block name and item labels for "Chưa tới lịch tiêm" ("Not yet scheduled"); pregnant live-vaccine items show "sau sinh — hỏi bác sĩ" ("after delivery — ask a doctor") | — (refines B2) | F05.4a, F05.7 |
| EQ19 | No birth date: only birth-anchored milestones replaced, keeping the gap between milestones; age validation skipped, interval checks kept; `NO_BIRTH_DATE` cases | B8 | F05.5 |
| EQ20 | Level-4 / implied doses: numbered sit at their number, unnumbered have no position; only the predecessor interval is waived; no feasible placement → `DOSE_INVALID`; first-dose stand-in `FIRST_DOSE_DATE_ASSUMED`; always estimated | — (refines B7b) | F04.1, F04.2, F05.10 |
| EQ21 | Eight-layer precedence; certainly out of age wins over history defaults, level-4 ⚪ and (conditionally) dose validity | B10 (fills SAD §5.3 step 10) | F03.6, F04.5, F05.4, F05.11 (new) |
| EQ22 | Auto-exit details: pending computed before suspension / pregnancy; evaluated only after the user saves; no birth date → button only; missing timestamp is invalid input | — (refines B11) | F03.6, F05.1 |
| EQ23 | Two kinds of disease declarations: history (yield to dose records) vs acknowledged (win while active); postpone expiry returns with banner; "chọn không tiêm lúc này" ("choose not now") waits for the re-ask; level-5 cut-off by creation date | — (corrects SAD §5.6 table C) | F03.6, F04.2, F04.6, §5 |
| EQ24 | Periodic boosters: start after the primary series, numbers continue; every-N-months and by-season formulas; season never 🔴; booster-only disease = one-dose series + booster; unknown last dose follows the rule | — | F05.2, F05.4, F05.12 (new), F08, §5 |
| EQ25 | Pregnancy: live products removed from the list; pre-pregnancy rule only adds a note; postpartum note lasts 3 months; `LIVE_VACCINE_BEFORE_PREGNANCY` | — | F05.3, F05.7 |
| EQ26 | Dose valid under the rule on its date or today; next dose uses today's rules; `NO_SERIES_IN_EFFECT`; start age required for several series per (disease, product); optional `catchUpAfterAge` → `CATCH_UP_ASK_DOCTOR` | — | F05.1, F05.2, F05.10, §5 |
| EQ27 | Wording clarifications: display constants need no citation; tolerance only for validation; any alternative plan validates a dose; same-product is a disease-level rule and the switch warning needs ≥ 2 known products; safest-series explanation lists each series; separate input validation, `INVALID_DATE` | — | F04.3, F05.1, F05.2, F05.3, F05.10, §5 |

Notes:
- EQ3, EQ4 and EQ9 are kept for traceability only: EQ3 and EQ4 are superseded by EQ15, and EQ9 by EQ19.
- EQ21 places "certainly out of age" (layer 5) above the history-status defaults (layer 6), reversing the earlier order "NOT_ENTERED before max age" (SAD v1.0 §5.3 step 7; now SAD §5.3 step 10).
- F13.2 gains the test cases listed for EQ1–EQ25, and two existing cases are reworded ("≥ 1 dose given" for ✅ "Đang theo phác đồ" ("On series"); out-of-age diseases for the 45-year-old adult).
