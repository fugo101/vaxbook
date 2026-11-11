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

**Disease declaration** — *Khai báo theo bệnh*:
What the user states about one disease for a profile that is not a dose record: unknown, confirmed not vaccinated, already immune, doctor advised against vaccination, postponed until a date, chose not to vaccinate for now. The first two are *history* declarations, and a dose record for the disease takes precedence over them. The other four are *acknowledged* declarations: while active they move the disease into the acknowledged block, even when dose records exist.
_Avoid_: ProfileDiseaseStatus, Disease status, Acknowledgement

**Pregnancy status** — *Tình trạng mang thai*:
A profile's "đang mang thai" ("pregnant") flag (with estimated delivery date) or "dự định mang thai" ("planning a pregnancy") flag (with a reconfirmation date).

## Catalog & rule set

**Disease** — *Bệnh*:
A vaccine-preventable disease; the unit that series and assessments are computed per.

**Product** — *Sản phẩm*:
A specific commercial vaccine; it may protect against several diseases (combination vaccine dose).
_Avoid_: Vaccine (when you mean a specific product), Brand

**Series** — *Phác đồ*:
A sequence of doses for one disease, optionally tied to one product and one start-age range.
_Avoid_: Schedule, Regimen, Protocol

**Dose rule** — *Quy tắc mũi*:
The conditions for one dose in a series: recommended milestone, minimum / maximum age, minimum interval, buffer, tolerance.

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
The last day a next dose can still be given so that the series can be completed before its maximum age.
_Avoid_: Due date, Max age

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
