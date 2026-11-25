# VaxBook

A neutral vaccination record for users in Vietnam: it stores the vaccination history of the user and their family members, and gives cited recommendations on which vaccines to consider. Each entry gives the **canonical term (used in code)** — *Vietnamese term (used in the UI)*.

## Users & profiles

**Account** — *Tài khoản*:
The login identity, identified by email only. Contains no medical data.
_Avoid_: User (when you mean an account), Customer

**Guest** — *Khách*:
A user with no account, or with an account who has not yet consented (or has withdrawn consent) to storing vaccination data; the data lives only on the device.
_Avoid_: Anonymous user, Offline mode

**Profile** — *Hồ sơ*:
One person whose vaccinations are tracked. Exists independently of accounts; an account links to a profile through a Membership.
_Avoid_: Member, Patient, Person, Child

**Membership**:
The relationship between one Account and one Profile. It carries the role (the MVP has only the owner role), the "Tôi" ("Me") flag and the reminder settings.
_Avoid_: Ownership, Link

**Self profile** — *Hồ sơ "Tôi"*:
The profile that the account's Membership marks as the user themselves; at most one per account.
_Avoid_: Main profile, Owner profile

**History status** — *Tình trạng lịch sử tiêm*:
How complete the vaccination history is that the user declared for a profile: "chưa nhập xong" ("not finished entering") / "không nhớ thời nhỏ" ("don't remember childhood") / "chưa tiêm mũi nào" ("no doses yet") / "đã nhập xong" ("finished entering"), with the time it was chosen.
_Avoid_: Onboarding state, Completeness

## Vaccination history

**Dose record** — *Mũi tiêm*:
One vaccination that took place, declared by the user, linked to at least one disease; the product is optional.
_Avoid_: Vaccination, Shot, Injection, Dose (when you mean the record)

**Date precision** — *Độ chính xác ngày*:
How precisely the user remembers a date: exact day / month-year / year only / time unknown (for a dose record), or not entered (for the birth date).
_Avoid_: Fuzzy date, Approximate date

**Recalled date** — *Ngày theo trí nhớ*:
A date as the user remembers it (a birth date or a dose date), together with its Date precision.
_Avoid_: Fuzzy date, Approximate date, Estimated date

**Dose number** — *Số thứ tự mũi*:
The position of a dose record in the series of one of its diseases. The engine infers it per disease from chronological order; the user may enter it, and an entered number places the record at that position.
_Avoid_: Dose index, Shot number

**Implied dose** — *Mũi ngầm định*:
A dose with no dose record that the engine counts because the user entered a higher dose number on a later record: it is counted and undated, like a level-4 dose.
_Avoid_: Missing dose, Assumed dose, Virtual dose

**Disease declaration** — *Khai báo theo bệnh*:
What the user states about one disease for a profile that is not a dose record: unknown, confirmed not vaccinated, already immune, doctor advised against vaccination, postponed until a date, chose not to vaccinate for now. The first two are *history* declarations, and a dose record for the disease takes precedence over them. The other four are *acknowledged* declarations: while active they move the disease into the acknowledged block, even when dose records exist.
_Avoid_: ProfileDiseaseStatus, Disease status, Acknowledgement

**Pregnancy status** — *Tình trạng mang thai*:
A profile's "đang mang thai" ("pregnant") flag (with estimated delivery date) or "dự định mang thai" ("planning a pregnancy") flag (with a reconfirmation date).

## Catalog & rule set

**Disease** — *Bệnh*:
A vaccine-preventable disease; the unit that series and assessments are computed per.

**Product** — *Sản phẩm*:
A specific commercial vaccine in the catalog; it may protect against several diseases (combination vaccine dose). It lists its diseases, carries a live-vaccine flag, and may be retired.
_Avoid_: Vaccine (when you mean a specific product), Brand

**Retired product** — UI label *"Không còn trong danh mục"* ("No longer in the catalog"):
A product that is no longer given but stays in the catalog with its series. The engine treats a dose with it as a dose with an unknown product, and never offers it for a dose not yet given.
_Avoid_: Deleted product, Discontinued product

**Off-catalog product** — *Sản phẩm ngoài danh mục*:
A product the user names in free text because it is not in the catalog. The engine reads only that one was named; a dose with it counts as a dose with an unknown product.
_Avoid_: Unknown product (which also covers a dose with no product), Custom product

**Series** — *Phác đồ*:
A sequence of doses for one disease, optionally tied to one product and one start-age bracket.
_Avoid_: Schedule, Regimen, Protocol

**Generic series**:
A series with no product: it applies to every product of its disease that has no series of its own.
_Avoid_: Default series, General series

**Product group**:
The series of one disease that share one product, or the generic series of a disease. Start-age brackets tell the series of one group apart.
_Avoid_: Product series (one series), Series family

**Start-age bracket**:
The ages at dose 1 a series applies to: a half-open range [min, max), with max optional (open-ended). The brackets of one product group do not overlap; the engine chooses a group's series by the age at dose 1, or, with no dose yet, by the age today. For choosing, a bracket's _reach_ also covers the gap below it, from the previous bracket's max (from birth for the first), so every age below the last max is in exactly one reach (Q33). The last bracket's max also limits dose 1, like a maximum age to start, for every series of the group (Q55). Not the minimum age of a dose.
_Avoid_: Age range (ambiguous with the minimum and maximum ages), Age group

**Maximum age to start** — *Tuổi tối đa bắt đầu*:
The age before which dose 1 of a series must be given (`maxAgeStart`), an exclusive bound: the last allowed day is birth + the maximum age − 1 day. It belongs to the series, not to one plan, and applies to dose 1 only; the last start-age bracket max of the product group limits dose 1 too, whichever is earlier. A dose 1 certainly on or after it is not counted (`DOSE_AFTER_MAX_AGE`).
_Avoid_: Start age (the bracket's min), Age limit (ambiguous)

**Maximum age to complete** — *Tuổi tối đa hoàn thành*:
The age before which every dose of a series must be given (`maxAgeComplete`), an exclusive bound, series-level like the maximum age to start. A dose certainly on or after it is not counted.
_Avoid_: Last allowed day (the day derived from it for one next dose)

**Validity window** — *Thời gian hiệu lực*:
The days a series is in effect: a half-open range [`validFrom`, `validTo`), either bound optional (unbounded when absent). A successor that replaces a series starts on the day the old one ends. The engine schedules the next dose with the series in effect today, and accepts a given dose that met the series in effect on its date (a version).
_Avoid_: Effective period (ambiguous with a dose date range), Validity range

**Validity segment**:
A stretch of time in which the same series of a disease are in effect: the validity windows of its series cut time at their bounds. The engine compiles each segment once and evaluates on the one that contains today; a series of another segment that may count a given dose is a _version_ of today's series.
_Avoid_: Period, Era

**Catch-up age** — *Tuổi tiêm bù*:
The age of a series (`catchUpAfterAge`) from which a late next dose is handed to a doctor instead of being scheduled: when the earliest date the result would show is at or after birth + the catch-up age, the result is ⚪ `CATCH_UP_ASK_DOCTOR`. Not a late start of a series in a start-age bracket.
_Avoid_: Catch-up series (a series kind not modelled yet), Maximum age (a dose past it is not counted; past the catch-up age it may still count)

**Named product**:
For choosing between the reasons `CHECK_PRODUCT_NAME` and `MIXED_PRODUCTS` only: a dose with a catalog product that lists the disease (retired or not) or with an off-catalog product, as opposed to a dose with no product, a catalog product that does not list the disease, or an implied dose. Not the same as the products that decide the candidate series, where a retired or off-catalog product counts as unknown.
_Avoid_: Known product (ambiguous with the candidate rule)

**Candidate series**:
A series the engine still considers for one disease of a profile after reading the products of its dose records; with no dose given, every series except those of retired products.
_Avoid_: Possible series, Matching series

**Safest series**:
The result over several candidate series: each date of the next dose takes its latest value among them, and the disease is complete only when every candidate is.
_Avoid_: Strictest product, Worst case

**Dose rule** — *Quy tắc mũi*:
The conditions for one dose in a series: recommended milestone, minimum age, minimum intervals, buffer, tolerance. The maximum ages belong to the series, not to a dose rule.

**Tolerance** — *Dung sai*:
A sourced grace period of a dose rule, in days or weeks: a given dose meets the rule's minimum age and minimum intervals when it is at most the tolerance early. It is used only to validate doses already given, never to compute a next-dose date or limit, and never lowers a maximum age.
_Avoid_: Grace period (ambiguous with the overdue grace), Buffer (moves only the overdue date)

**Rule** — *Quy tắc đặc biệt*:
A rule that does not belong to a specific dose: periodic booster, before pregnancy, allowed additional dose, same-product requirement, information-only.
_Avoid_: using "quy tắc" ("rule") loosely when you mean Series or Dose rule

**Rule set version** — *Phiên bản bộ quy tắc*:
The whole catalog and all rules at one publish; immutable once created.
_Avoid_: Release, Dataset version

**TCMR label** — *Nhãn TCMR*:
Marks a product as given free of charge under Tiêm chủng mở rộng (TCMR, Vietnam's Expanded Programme on Immunization) for a target group, with effective dates.
_Avoid_: Free flag, EPI flag

**Citation** — *Trích dẫn*:
A link from a catalog item or rule to a specific location (article / clause / page) in a Source document.

**Suspension** — *Tạm ngưng tính lịch*:
An emergency admin order that makes every assessment based on one Series, one Rule or a whole Disease become "chưa rõ" ("unknown"), unless the disease is information-only or has an active acknowledged declaration; it persists across all versions until it is lifted. It does not apply to individual Dose rules.
_Avoid_: Disable, Kill switch

## Data editing

**Changeset**:
A bundle of proposed changes to the catalog and rule set. It goes through the lifecycle Nháp (Draft) → Chờ duyệt (Pending review) → Đã duyệt (Approved) → Đã xuất bản (Published) (or Từ chối (Rejected) / Lỗi thời (Stale)).
_Avoid_: Patch, Draft, PR

**Revision** — *Bản sửa*:
A numbered, locked version of a changeset at the time it is submitted for review.

**Finding** — *Phát hiện*:
An issue raised by an AI review role on a revision, with a severity and a source.
_Avoid_: Issue, Comment

**Test case** — *Ca kiểm thử*:
A simulated profile with the expected assessment result, used to test the recommendation engine on a revision.

## Recommendation results

**Assessment** — *Đánh giá* (UI: "gợi ý" ("recommendation")):
The recommendation engine's conclusion for one (profile, disease) pair at one calculation date and one rule set version; it falls in exactly one Bucket or one Block.
_Avoid_: Recommendation, Status, Result

**Next dose** — *Mũi kế tiếp*:
The not-yet-given dose that the recommendation engine identifies for one disease of one profile, identified by (profile, disease, dose number for that disease); for periodic boosters, the dose number keeps increasing.
_Avoid_: Upcoming dose, Due dose, Reminder (when you mean a dose)

**Earliest allowed date** — *Ngày sớm nhất được phép*:
The first day a next dose may be given, satisfying its minimum age and minimum intervals.

**Due date** — *Ngày đến hạn*:
The day a next dose is recommended; never before the earliest allowed date and, while the dose can still be given in time, never after the last allowed day.
_Avoid_: Deadline

**Overdue date** — *Ngày quá hạn*:
The day from which a next dose is shown as overdue, normally at least 7 days after the due date. A dose held to its last allowed day, and a seasonal booster, are never shown as overdue.

**Last allowed day** — *Hạn chót*:
The last day a next dose can still be given so that the series can be completed before its maximum age (for dose 1, also before its maximum age to start). Past it, a dose that can still be given before the maximum age itself is shown as "có thể không kịp đủ phác đồ — hỏi bác sĩ" ("may not be able to complete the series — ask a doctor"), held to that later day; a dose that cannot is out of age.
_Avoid_: Due date, Max age

**Counted view**:
The engine's second look at a disease's dose records with every record counted, used only to decide whether a disease is out of age, so that a dose the engine questions never makes a person out of age by not being counted.
_Avoid_: Corrected history, Assumed history

**Certainly out of age** — *Chắc chắn quá tuổi*:
Out of age at the young end of the birth-date range and from the earliest possible dose dates (an imprecise date's earliest day; an undated or implied dose 1 never stands in for a known date). Only then is a disease shown as no longer within vaccination age; when a more exact dose date could still allow the next dose, the engine asks for that date instead.
_Avoid_: Probably out of age, Past the last allowed day

**Periodic booster** — *Nhắc lại định kỳ*:
A dose repeated after a series is complete, either every fixed period or once per season; its dose numbers continue those of the series.
_Avoid_: Revaccination, Annual dose

**Bucket** — *Nhóm*:
One of the five main buckets: overdue (quá hạn), due (đến hạn), upcoming (sắp tới), unknown (chưa rõ), complete (đã hoàn thành).
_Avoid_: Category, Status

**Block** — *Khối*:
An area outside the five buckets, never shown in red and never emailed: not yet scheduled (chưa tới lịch tiêm), no longer within vaccination age (không còn trong độ tuổi tiêm), archive only (chỉ lưu trữ), acknowledged (đã ghi nhận), deferred during pregnancy (hoãn trong thai kỳ), during pregnancy (trong thai kỳ).
_Avoid_: Section, Bucket

## Privacy & notifications

**Consent record** — *Bản ghi đồng ý*:
One act of giving or withdrawing consent, with a type, a document version and a timestamp; at the account level or at the (account, profile) pair level.

**Disclaimer acceptance** — *Xác nhận miễn trừ*:
The act of a user (guests included) ticking the disclaimer text at a specific version.
_Avoid_: Consent (a disclaimer acceptance is not consent to data processing)

**Reminder email** — *Email nhắc lịch*:
An email grouped per account, at most one per day, listing the doses that are about to be due or that are 7 days overdue.
_Avoid_: Notification (when you mean an email)

**Notification** — *Thông báo*:
One item in an account's in-app notification center.

**Feedback report** — *Báo sai dữ liệu*:
An anonymous report about a catalog item, rule or table cell that may be wrong; not linked to an account or profile.
_Avoid_: Bug report, Ticket
