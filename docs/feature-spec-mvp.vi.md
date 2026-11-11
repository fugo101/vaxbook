# VaxBook — Đặc tả tính năng MVP

> **Bản tiếng Việt để tham khảo.** Bản gốc là [`feature-spec-mvp.md`](feature-spec-mvp.md) (tiếng Anh); khi hai bản lệch nhau, bản tiếng Anh có hiệu lực. Chuỗi giao diện (UI) tiếng Việt trong bản gốc được giữ nguyên văn.

| | |
|---|---|
| **Phiên bản** | 1.2 |
| **Ngày** | 2026-11-11 |
| **Trạng thái** | Đã chốt bước tính năng (Q1–Q54, 2 vòng rà soát). v1.1: chỉnh theo SAD (xem Phụ lục B). v1.2: chốt ngữ nghĩa bộ máy gợi ý (Phụ lục C) |
| **Tài liệu liên quan** | [`docs/research/2026-11-10-feature-research.md`](research/2026-11-10-feature-research.md), [`docs/sad.md`](sad.md), [`GLOSSARY.md`](../GLOSSARY.md) |

> Tài liệu này chỉ mô tả **tính năng** (làm gì, quy tắc nghiệp vụ, phạm vi). Kiến trúc, công nghệ, hạ tầng thuộc SAD.
> Các mục ghi **(CHƯA KIỂM CHỨNG)** dựa trên nghiên cứu chưa được xác nhận từ nguồn gốc.

---

## 1. Tổng quan

### 1.1 Mục tiêu
VaxBook là web app giúp người dùng tại Việt Nam:
1. Lưu lịch sử tiêm chủng của bản thân và người thân.
2. Biết **hiện tại nên cân nhắc tiêm gì**, dựa trên tuổi, giới tính, lịch sử đã tiêm và bảng lịch tiêm theo độ tuổi.
3. Tra cứu danh mục bệnh, các dòng vaccine của từng bệnh và bảng tiêm theo độ tuổi.

### 1.2 Định vị
- **Phi lợi nhuận**, công cụ hỗ trợ cá nhân, do một cá nhân vận hành.
- Thông tin **chỉ mang tính tham khảo, gợi ý**. Quyết định tiêm thuộc về người dùng sau khi tham khảo ý kiến bác sĩ.
- Cố gắng xây dữ liệu chính xác nhất có thể, nhưng **không thay thế bác sĩ** và **không phải nguồn dữ liệu chuẩn**.
- Mức trách nhiệm có thể được nâng lên trong tương lai nếu có tổ chức đứng ra bảo trợ.

### 1.3 Người dùng mục tiêu
| Persona | Mô tả | Nhu cầu chính |
|---|---|---|
| **Cá nhân** | Người trung niên độc thân, chỉ dùng cho bản thân, thường không nhớ lịch sử tiêm thời nhỏ | Biết mình nên tiêm gì ở tuổi hiện tại, nhắc mũi nhắc định kỳ |
| **Bố mẹ** | Theo dõi lịch tiêm cho con nhỏ (0–6 tuổi là giai đoạn dày mũi nhất) | Không bỏ lỡ mốc, biết mũi nào miễn phí (TCMR) |
| **Người chăm sóc** | Theo dõi cho bố mẹ già, có thể không theo dõi cho chính mình | Gợi ý cho người cao tuổi (cúm, phế cầu, zona…) |

### 1.4 Khác biệt so với giải pháp hiện có
Các giải pháp hiện có (VNeID, hệ thống tiêm chủng quốc gia, app VNVC, app Long Châu, các app cũ ngừng cập nhật) xem chi tiết tại tài liệu nghiên cứu. VaxBook khác ở:
1. Sổ tiêm **trung lập**, gom mũi tiêm từ mọi nơi (TCMR, VNVC, Long Châu, nước ngoài, sổ giấy).
2. Gợi ý **có trích nguồn**, không gắn với việc bán hàng.
3. Xử lý được trường hợp **không nhớ ngày tiêm, không nhớ tên vaccine, không nhớ lịch sử thời nhỏ**.
4. Phủ cả người lớn, người cao tuổi, phụ nữ mang thai / dự định mang thai.

---

## 2. Nguyên tắc sản phẩm

| # | Nguyên tắc | Áp dụng |
|---|---|---|
| P1 | **An toàn khi không chắc chắn** | Khi dữ liệu không đầy đủ, không bao giờ gợi ý tiêm *sớm hơn* mức an toàn, và không gợi ý mũi có thể đã *quá tuổi tối đa*; đưa vào nhóm "Chưa rõ" thay vì đoán |
| P2 | **Luôn trích nguồn** | Mỗi gợi ý / quy tắc có nguồn và ngày cập nhật bộ quy tắc |
| P3 | **Tham khảo, không chỉ định** | Văn phong "có thể cân nhắc", luôn kèm "hỏi bác sĩ" |
| P4 | **Thu thập tối thiểu** | Chỉ thu những gì cần; mọi trường nhân khẩu đều có thể bỏ trống |
| P5 | **Không làm user nản** | Không dội hàng loạt cảnh báo đỏ; gom mục chưa rõ vào một nhóm |
| P6 | **Thiết kế cho mở rộng** | Các tính năng hoãn (giá, push, chia sẻ, chuyển hồ sơ…) phải bổ sung được mà không phải migrate lớn |

---

## 3. Phạm vi

### 3.1 CÓ trong MVP

| Mã | Nhóm | Tính năng |
|---|---|---|
| F01 | Truy cập | Đăng nhập email OTP; chế độ khách; chuyển dữ liệu khách sang tài khoản; xoá; rút đồng ý; đổi email; tải dữ liệu của tôi |
| F02 | Bắt đầu sử dụng | Chọn đối tượng theo dõi; chấp nhận miễn trừ trách nhiệm; đồng ý xử lý dữ liệu |
| F03 | Hồ sơ | Nhiều hồ sơ / tài khoản; hồ sơ "Tôi" tuỳ chọn; giao diện tự suy ra cá nhân / gia đình; xác nhận đồng ý cho hồ sơ người lớn khác; tình trạng lịch sử khi tạo hồ sơ |
| F04 | Lịch sử tiêm | Nhập theo bệnh, sản phẩm tuỳ chọn, mũi phối hợp, độ chính xác ngày, "không nhớ thời nhỏ", kiểm tra dữ liệu nhập, ghi nhận theo bệnh sau khi hỏi bác sĩ |
| F05 | Bộ máy gợi ý | Gợi ý theo tuổi, giới, lịch sử, khoảng cách mũi, tiêm bù, phác đồ sản phẩm, nhắc định kỳ, mang thai |
| F06 | Bảng theo độ tuổi | Bản công khai + bản cá nhân hoá; dạng danh sách trên mobile |
| F07 | Danh mục | Bệnh, sản phẩm (không giá), link sang VNVC / Long Châu |
| F08 | Thông báo | Trung tâm thông báo trong app; email nhắc lịch gộp theo tài khoản; cài đặt kênh |
| F09 | Bản in | Trang in lịch sử tiêm của một hồ sơ |
| F10 | Minh bạch dữ liệu | Miễn trừ trách nhiệm 4 tầng; trang "Về dữ liệu"; nút "Báo sai dữ liệu" |
| F11 | Admin | Quản lý danh mục & quy tắc qua changeset; diff, duyệt, xuất bản, rollback; tạm ngưng tính lịch khẩn cấp; hàng chờ báo lỗi |
| F12 | MCP admin | Thêm / sửa / xoá dữ liệu dưới dạng changeset; chạy thử bộ máy gợi ý |
| F13 | AI rà soát | 5 vai trò chuyên môn; chặn xuất bản khi có lỗi mức cao |

Nền tảng: web **mobile-first**, **chỉ tiếng Việt**, PWA cơ bản (cài lên màn hình chính).

### 3.2 KHÔNG làm trong MVP
Các mục có trong §6 được thiết kế sẵn để bổ sung dễ; các mục còn lại (bản đồ trung tâm, tình trạng còn hàng, gói tiêm, app native) chưa có thiết kế sẵn.
- Giá theo nguồn & đồng bộ giá (VNVC, Long Châu, …)
- Web push
- Chia sẻ một hồ sơ cho nhiều tài khoản cùng quản lý
- **Tách một hồ sơ thành tài khoản riêng / chuyển hồ sơ sang tài khoản khác**
- Đăng nhập Google
- Ảnh sổ tiêm, OCR
- Đồng bộ từ VNeID / hệ thống tiêm chủng quốc gia
- Gợi ý theo bệnh nền, du lịch, nghề nghiệp
- Đặt lịch tiêm, bản đồ trung tâm, tình trạng còn hàng, gói tiêm
- Tiếng Anh, app native

---

## 4. Đặc tả chi tiết

### F01 — Truy cập & tài khoản

**F01.1 Đăng nhập email OTP**
- Tài khoản chỉ định danh bằng **email**. Không thu số điện thoại, họ tên thật, CCCD, địa chỉ.
- Đăng nhập không mật khẩu bằng mã OTP gửi qua email.
- Đăng xuất xoá mọi dữ liệu hồ sơ và lịch sử của tài khoản đang lưu tạm trên thiết bị.

**F01.2 Chế độ khách**
- Khách dùng được: tạo hồ sơ tạm, nhập lịch sử, xem gợi ý, bảng theo độ tuổi, danh mục.
- Dữ liệu khách **chỉ lưu trên trình duyệt**; khách không nhận email nhắc lịch và không có trung tâm thông báo (chỉ banner).
- Hiển thị rõ: "Dữ liệu chỉ lưu trên thiết bị này, có thể mất nếu xoá dữ liệu trình duyệt. Trình duyệt có thể tự xoá nếu lâu không mở hoặc khi dùng chế độ ẩn danh; người khác dùng chung thiết bị có thể xem được."
- Cài đặt của khách luôn có nút **"Xoá toàn bộ dữ liệu trên thiết bị này"** (có xác nhận).

**F01.3 Chuyển dữ liệu khách sang tài khoản**
Khi khách đăng ký / đăng nhập mà trình duyệt còn dữ liệu khách:
1. Liệt kê từng hồ sơ khách (biệt danh, số mũi) có ô chọn, mặc định chọn hết.
2. Nếu tài khoản chưa có đồng ý "Lưu dữ liệu tiêm chủng" → hiện màn đồng ý (F02.3) **trước khi** tải lên; không đồng ý thì không tải gì.
3. Nếu tài khoản đã có hồ sơ "Tôi" và dữ liệu khách cũng có "Tôi" → hỏi "Hồ sơ nào là bạn?"; hồ sơ còn lại thành hồ sơ người thân (giữ biệt danh). Đảm bảo F03.2 (tối đa một "Tôi").
4. Không tự gộp các hồ sơ trùng biệt danh.
5. Hiện tóm tắt "Đã chuyển N hồ sơ, M mũi tiêm" rồi mới xoá bản trên trình duyệt.
- Việc chuyển là **tất cả hoặc không** cho các hồ sơ đã chọn; chạy lại không tạo bản trùng.
- Xác nhận miễn trừ trách nhiệm đã tick lúc là khách được giữ (nếu cùng phiên bản).
- Với mỗi hồ sơ được chọn **không phải "Tôi"** của người từ 18 tuổi hoặc chưa rõ tuổi (kể cả hồ sơ "Tôi" vừa chuyển thành người thân ở bước 3): hiện ô tick F03.5 cho từng hồ sơ trước khi tải lên và ghi bản ghi đồng ý (tài khoản, hồ sơ). Hồ sơ không được tick thì bị bỏ chọn và vẫn để trên thiết bị.
- Từ chối chuyển → khi đang đăng nhập không hiển thị dữ liệu khách; Cài đặt có mục "Dữ liệu khách trên thiết bị này" với nút "Chuyển vào tài khoản" và "Xoá". Quy tắc ẩn này chỉ áp dụng cho tài khoản **đang có** đồng ý "Lưu dữ liệu tiêm chủng" (xem F01.5).

**F01.4 Xoá dữ liệu**
- Xoá **từng hồ sơ**: xoá toàn bộ lịch sử tiêm và trạng thái của hồ sơ; huỷ ngay mọi email nhắc chưa gửi liên quan tới hồ sơ; xoá các thông báo của hồ sơ. Email gộp kế tiếp được tạo lại từ dữ liệu còn lại.
- Xoá **toàn bộ tài khoản**: như trên cho mọi hồ sơ mà tài khoản là chủ sở hữu duy nhất (MVP: mọi hồ sơ), cộng email, mọi thông báo, email chờ gửi, bản ghi đồng ý.
- Dữ liệu người dùng là **xoá cứng**; xoá mềm (F11.3) chỉ áp dụng cho danh mục và quy tắc.
- Nhật ký hệ thống chỉ lưu mã định danh nội bộ, không lưu email hay biệt danh.
- **Giai đoạn beta không có bản sao lưu** (SAD, ADR-0007): dữ liệu đã xoá mất ngay; đổi lại, sự cố máy chủ có thể làm mất dữ liệu. Trang "Về dữ liệu" nói rõ điều này và khuyên dùng "Tải dữ liệu của tôi" (F01.7) định kỳ. Khi có sao lưu (điều kiện trước public), trang này ghi thời hạn dữ liệu đã xoá còn tồn tại trong bản sao lưu.
- Cả hai thao tác yêu cầu xác nhận lại; sau khi xoá không khôi phục được. Hộp thoại xác nhận có nút **"Tải về trước"** (F01.7).

**F01.5 Rút đồng ý "Lưu dữ liệu tiêm chủng"**
- Rút đồng ý → xoá toàn bộ hồ sơ và lịch sử tiêm trên server (như F01.4), **giữ tài khoản** (chỉ còn email và cài đặt).
- Trước khi rút, user được chọn **giữ một bản trên thiết bị** dưới dạng dữ liệu khách (F01.2); hộp thoại có nút "Tải về trước" (F01.7).
- Sau khi rút đồng ý (hoặc khi chưa tick đồng ý lưu theo F02.3), user **vẫn đăng nhập nhưng dùng như chế độ khách**: dữ liệu khách trên thiết bị, kể cả bản vừa giữ lại, được hiển thị và dùng như F01.2. Đăng xuất **không** xoá bản này.
- Muốn lưu lại trên tài khoản thì phải đồng ý lại → chạy luồng chuyển F01.3.
- Cài đặt có mục **"Quyền riêng tư"** liệt kê từng đồng ý (lưu dữ liệu, email nhắc lịch, đồng ý theo từng hồ sơ người thân — F03.5) kèm ngày đồng ý, và cho rút từng mục. Mỗi lần đồng ý / rút đều ghi bản ghi đồng ý có phiên bản văn bản.
- Rút đồng ý theo **một hồ sơ** (F03.5) → xoá hồ sơ đó trên server như xoá từng hồ sơ (F01.4); hộp thoại có nút "Tải về trước" (F01.7).

**F01.6 Đổi email**
- Khi đang đăng nhập: nhập email mới → xác nhận bằng OTP gửi tới email mới → gửi thông báo trung tính tới email cũ.
- Người vận hành **không khôi phục tài khoản thủ công** (khó xác minh danh tính, dễ bị giả danh để lấy dữ liệu sức khoẻ). Ghi rõ điều này ở trang "Về dữ liệu" và trong Cài đặt.

**F01.7 Tải dữ liệu của tôi**
- Xuất toàn bộ hồ sơ và lịch sử tiêm ra một **tệp có cấu trúc**; dùng được cho cả khách.
- Nút "Tải về trước" xuất hiện trong hộp thoại xoá hồ sơ, xoá tài khoản và rút đồng ý.

### F02 — Bắt đầu sử dụng (onboarding)

1. **"Bạn muốn theo dõi lịch tiêm cho ai?"** — chọn nhiều: `Bản thân` / `Người thân (con, bố mẹ…)`.
   - Lựa chọn chỉ quyết định **bước tạo hồ sơ tiếp theo**, không tạo ra "chế độ" nào (xem F03.4).
2. **Miễn trừ trách nhiệm** — bắt buộc tick *"Tôi hiểu thông tin chỉ để tham khảo, không thay thế tư vấn của bác sĩ"* trước khi xem gợi ý cá nhân. Áp dụng cả với khách.
   - Bảng theo độ tuổi bản công khai (F06) và danh mục (F07) **không cần tick**; bản cá nhân hoá F06 và trang gợi ý chỉ hiển thị sau khi đã tick.
   - Văn bản miễn trừ có **số phiên bản**; khi phiên bản đổi, user (kể cả khách) phải tick lại trước khi xem gợi ý.
3. **Đồng ý xử lý dữ liệu** (chỉ khi có tài khoản) — tách riêng từng mục, không tick sẵn:
   - Lưu dữ liệu tiêm chủng (bắt buộc để lưu trên tài khoản). Văn bản nói rõ **đây là thông tin sức khoẻ (dữ liệu nhạy cảm)**, và với hồ sơ trẻ em: *"Tôi là cha mẹ / người giám hộ của trẻ em mà tôi tạo hồ sơ"*.
   - Nhận email nhắc lịch (tuỳ chọn).
   - **Không tick** "Lưu dữ liệu tiêm chủng" → tiếp tục dùng ở **chế độ khách** (F01.2). Rút đồng ý về sau: xem F01.5.
4. Tạo hồ sơ đầu tiên (F03), gồm câu hỏi tình trạng lịch sử tiêm (F03.6).

### F03 — Hồ sơ người được tiêm

**F03.1 Trường thông tin**
| Trường | Bắt buộc | Ghi chú |
|---|---|---|
| Biệt danh | Có | Ví dụ "Tôi", "Mẹ", "Bé Na". Không cần họ tên thật |
| Ngày sinh | Không | 4 mức: đủ ngày / tháng-năm / chỉ năm / không nhập |
| Giới tính | Không | Nam / Nữ / để trống |
| Cờ mang thai | Không | Xem F05.7 |
| Tình trạng lịch sử tiêm | Có | Xem F03.6 (lựa chọn "Không nhớ lịch sử thời nhỏ" → F04.5) |

**F03.2 Hồ sơ "Tôi"**
- Tuỳ chọn: chỉ tạo khi user chọn "Bản thân" ở onboarding hoặc thêm sau trong cài đặt.
- Mỗi tài khoản có **tối đa một** hồ sơ được đánh dấu "Tôi".
- Hồ sơ "Tôi" xưng "bạn"; hồ sơ người thân dùng biệt danh ("Bé Na nên tiêm…").

**F03.3 Độ đầy đủ thông tin**
- Mỗi hồ sơ hiển thị chỉ báo "Độ đầy đủ thông tin" và gợi ý trường cần bổ sung để gợi ý chính xác hơn.
- Với trẻ dưới 2 tuổi chưa có ngày sinh đầy đủ: nhắc nhẹ "Nên nhập ngày sinh đầy đủ cho trẻ nhỏ" (không bắt buộc).

**F03.4 Giao diện cá nhân / gia đình — tự suy ra, không có công tắc**
- **1 hồ sơ** → giao diện cá nhân: vào thẳng trang gợi ý của hồ sơ đó; không có thanh chuyển hồ sơ; link nhỏ "+ Thêm người thân".
- **≥ 2 hồ sơ** → giao diện gia đình: trang "Tổng quan gia đình" với thẻ từng người, sắp theo mức độ cần chú ý (số mục quá hạn / đến hạn); có thanh chuyển hồ sơ. Các mục ⚪ "Chưa nhập" (F03.6) và các mục ở khối ngoài 5 nhóm (F05.4a, kể cả "Chưa tới lịch tiêm") **không tính** vào mức độ cần chú ý.
- Xoá bớt về còn 1 hồ sơ → tự quay về giao diện cá nhân.
- Cài đặt có mục **"Người thân"** để thêm / sửa / xoá hồ sơ.

**F03.5 Xác nhận khi tạo hồ sơ người lớn khác**
- Khi tạo hồ sơ **không phải "Tôi"** cho người **từ 18 tuổi** (hoặc chưa rõ tuổi): ô tick bắt buộc, không tick sẵn — *"Tôi là người giám hộ, hoặc đã được người này đồng ý cho lưu thông tin tiêm chủng"*.
- Hồ sơ trẻ em dùng xác nhận chung ở F02.3.
- Lưu **bản ghi đồng ý theo cặp (tài khoản, hồ sơ)** ngay từ MVP — cũng là nền cho việc chuyển / tách hồ sơ sau này (§6).

**F03.6 Tình trạng lịch sử tiêm khi tạo hồ sơ** (phân biệt "chưa nhập" với "chưa tiêm")
- Câu hỏi **bắt buộc** khi tạo hồ sơ, ba lựa chọn:
  1. **"Đã có mũi tiêm, tôi sẽ nhập"** → hồ sơ ở trạng thái **"Chưa nhập xong"**: chỉ các mốc có ngày đến hạn **trước thời điểm user chọn lựa chọn này** mà chưa có bản ghi mới hiện ⚪ "Chưa nhập — kiểm tra sổ tiêm", không thành 🔴. Mốc đến hạn sau thời điểm đó được tính bình thường theo F05.4. Hồ sơ thoát trạng thái này khi user bấm **"Tôi đã nhập xong"**, hoặc tự động khi **không còn đánh giá nào** của hồ sơ là ⚪ "Chưa nhập" ở bất kỳ đầu nào của khoảng ngày sinh. Việc này xét **theo từng mốc, không theo từng bệnh**: nhập mũi 1 của phác đồ 3 mũi không làm thoát trạng thái khi mũi 2 vẫn là ⚪ "Chưa nhập".
  - Một bệnh hết chờ nhập khi không còn mốc nào của nó là ⚪ "Chưa nhập": các mũi đã được ghi, bệnh có khai báo về lịch sử (F04.2 mức 5, nút nhanh bên dưới), có khai báo ghi nhận đang hiệu lực (F04.6), hoặc chắc chắn đã quá tuổi (F05.11 tầng 5). Các mốc sau một mũi mức 4 không tính là chờ nhập.
  - Bộ máy trả về danh sách các bệnh còn chờ nhập, để server và client cùng quyết định từ một trường. Danh sách này được tính trước khi áp dụng tạm ngưng (F11.8) và mang thai (F05.7), nên hai yếu tố này không giữ lại cũng không giải phóng bệnh nào.
  - Việc tự động thoát **chỉ được xét ngay sau khi user lưu** một mũi tiêm hoặc một khai báo theo bệnh — không bao giờ khi tính lại thụ động (phiên bản bộ quy tắc mới, rollback, tạm ngưng, thời gian trôi qua).
  - Hồ sơ không có ngày sinh chỉ thoát trạng thái này bằng nút "Tôi đã nhập xong".
  - Hồ sơ ở trạng thái "Chưa nhập xong" phải có thời điểm chọn lựa chọn này; thiếu thời điểm thì dữ liệu đầu vào không hợp lệ (F05.1).
  2. **"Không nhớ lịch sử thời nhỏ"** → F04.5.
  3. **"Chưa tiêm mũi nào"** → bộ máy coi là chưa tiêm, tính bình thường.
- **Hồ sơ người lớn đã qua tuổi phác đồ thơ ấu, ở lựa chọn 1 hoặc 2** (lựa chọn 3 tính bình thường, có thể có 🔴): với mốc đã qua của các bệnh còn trong độ tuổi tiêm và chưa có bản ghi, hiện ⚪ "Bạn đã từng tiêm chưa?" (ở lựa chọn 1 thay cho nhãn "Chưa nhập"; ở lựa chọn 2 nằm trong nhóm "Nên hỏi bác sĩ hoặc xét nghiệm" của F04.5) kèm nút nhanh **Đã tiêm / Chưa tiêm / Không nhớ**. Chỉ thành 🔴 khi user chọn "Chưa tiêm". "Đã tiêm" tạo mũi mức 4; "Không nhớ" tạo khai báo theo bệnh "chưa rõ" (mức 5); "Chưa tiêm" tạo khai báo theo bệnh **"đã xác nhận chưa tiêm"** → bộ máy tính bình thường (có thể 🔴). Huỷ được như ghi nhận F04.6. Hai khai báo này là **khai báo về lịch sử** (F04.6): khi bệnh đó có mũi tiêm thì dữ liệu mũi tiêm được ưu tiên; ngược lại, chúng được ưu tiên hơn tình trạng lịch sử. Các bệnh chắc chắn đã quá tuổi (ví dụ Rota, Hib ở người lớn) không có nút nhanh và vào khối "Không còn trong độ tuổi tiêm" (F05.11).
- Đổi được lựa chọn này trong cài đặt hồ sơ.

### F04 — Lịch sử tiêm

**F04.1 Nhập một mũi tiêm**
| Trường | Bắt buộc | Ghi chú |
|---|---|---|
| Bệnh | Có | Chọn một hoặc nhiều bệnh |
| Sản phẩm | Không | Chọn sản phẩm trong danh mục → hệ thống điền sẵn các bệnh từ quan hệ sản phẩm–bệnh (ví dụ 6in1, 5in1, MMR); user được bỏ bớt hoặc thêm |
| Ngày tiêm + độ chính xác | Có (độ chính xác) | Xem F04.2 |
| Số thứ tự mũi | Không | Tự suy **theo từng bệnh** của mũi đó từ thứ tự thời gian (ví dụ mũi 6in1 đầu tiên là mũi 2 viêm gan B nhưng mũi 1 Hib); user sửa được. Số mũi user nhập lớn hơn số bản ghi → coi như có các mũi trước ở mức 4 (**mũi ngầm định**, F04.2) |
| Nơi tiêm | Không | Văn bản tự do hoặc chọn (TCMR / VNVC / Long Châu / nước ngoài / khác) |
| Số lô | Không | |
| Ghi chú | Không | |

- Cho phép mũi tiêm có sản phẩm **ngoài danh mục** (ví dụ tiêm ở nước ngoài): nhập tên tự do và tự chọn các bệnh tương ứng.
- Khi nhập mũi cho bệnh **chỉ thông tin** (ví dụ dại): hiện thông báo không bỏ qua được — *"VaxBook không nhắc lịch cho bệnh này, hãy theo lịch hẹn của cơ sở y tế"*. Mũi được lưu vào lịch sử và hiện trong khối "Chỉ lưu trữ" (F05.4a).
- **Không** upload ảnh sổ tiêm trong MVP.

**F04.2 Độ chính xác ngày tiêm (5 mức)**
| Mức | Ý nghĩa | Xử lý trong bộ máy gợi ý |
|---|---|---|
| 1 | Đúng ngày | Tính bình thường |
| 2 | Chỉ nhớ tháng/năm | Coi là **cuối tháng** (không quá hôm nay) khi tính mũi kế tiếp |
| 3 | Chỉ nhớ năm | Coi là **cuối năm** (không quá hôm nay) khi tính mũi kế tiếp |
| 4 | Không nhớ thời gian, chắc chắn đã tiêm | Tính là đã có mũi. Nếu đây có thể là mũi gần nhất thì không tính được hạn mũi sau → ⚪ "Cần kiểm tra sổ tiêm / hỏi bác sĩ", trừ khi bệnh chắc chắn đã quá tuổi (F05.11). Với mũi nhắc lại định kỳ sau một mũi cuối ở mức 4, áp dụng hành vi của quy tắc khi mũi cuối không rõ (F05.12). Mũi có ngày tiêm sau nó được tin là hợp lệ (F05.10) |
| 5 | Không nhớ đã tiêm hay chưa | Không tạo bản ghi mũi tiêm; tạo **khai báo theo bệnh = "chưa rõ"** cho hồ sơ (một khai báo về lịch sử, F04.6; DiseaseDeclaration, §5) → gợi ý kèm ghi chú bác sĩ có thể chỉ định xét nghiệm kháng thể hoặc tiêm bù. Khai báo chỉ áp dụng cho các mốc có ngày đến hạn **trước ngày tạo khai báo**; các mốc sau được tính bình thường |

Mọi gợi ý dựa trên dữ liệu mức 2–5, hoặc có mũi mức 4 hay mũi ngầm định ở bất kỳ đâu trong phác đồ, gắn nhãn **"⚠️ dựa trên thông tin ước lượng"**.

Quy ước ngày ước lượng:
- Mũi mức 2–3 có khoảng ngày **[đầu kỳ, min(cuối kỳ, hôm nay)]**. Khoảng đã giới hạn tới hôm nay này được dùng chung cho việc xếp thứ tự, kiểm tra mũi hợp lệ (F05.10) và tính mũi kế tiếp.
- Ngày quy đổi khi tính mũi kế tiếp = điểm cuối của khoảng đó = **min(cuối tháng / cuối năm, hôm nay)**.
- Khi xếp thứ tự các mũi, mũi ước lượng dùng khoảng của nó; nếu chồng lấn với mũi khác thì dùng số thứ tự user nhập, không có thì kết quả của bệnh là ⚪. Các số thứ tự user nhập mâu thuẫn nhau → ⚪ `NUMBERING_CONFLICT`.
- Mũi mức 4 và mũi ngầm định: mũi mức 4 **có số thứ tự** nằm đúng ở số thứ tự đó. Mũi mức 4 **không có số thứ tự** được tính vào số mũi đã tiêm nhưng không có vị trí trong thứ tự thời gian; nếu nó có thể là mũi gần nhất thì mũi kế tiếp là ⚪ `PREVIOUS_DOSE_DATE_UNKNOWN` (theo thứ tự ưu tiên, F05.11).
- Nếu mũi 1 ở mức 4 hoặc là mũi ngầm định mà có quy tắc tính từ mũi đầu (ví dụ HPV 0-2-6), thì mũi có ngày tiêm sớm nhất được dùng thay cho ngày mũi đầu (cách này chỉ có thể đẩy các ngày muộn hơn, nên an toàn theo P1); kết quả gắn nhãn ước lượng, lý do `FIRST_DOSE_DATE_ASSUMED`.

**F04.3 Đổi loại vaccine giữa phác đồ**
- Cảnh báo "Đổi loại vaccine giữa chừng — hỏi bác sĩ" **chỉ hiện** khi bệnh có quy tắc **"yêu cầu cùng sản phẩm"** (một quy tắc đặc biệt cấp bệnh, admin nhập kèm nguồn, ví dụ Rota), và chỉ khi trong các mũi của bệnh không phải mũi bổ sung được phép có từ hai sản phẩm **đã biết** (trong danh mục hoặc đã ngừng) khác nhau trở lên. Một sản phẩm không rõ hoặc ngoài danh mục tự nó không gây cảnh báo. Cảnh báo chỉ mang tính thông tin, không làm đổi nhóm.
- Các trường hợp kết hợp hợp lệ (ví dụ viêm gan B sơ sinh đơn giá + 6in1) không bị cảnh báo (xem "mũi bổ sung được phép", F05.2).

**F04.4 Sửa / xoá mũi tiêm và kiểm tra dữ liệu nhập**
- Sửa, xoá từng mũi (xoá có xác nhận); gợi ý được tính lại ngay.
- **Chặn lưu** khi: ngày tiêm ở tương lai; ngày tiêm chắc chắn trước ngày sinh (so theo độ chính xác của cả hai).
- **Cho lưu kèm cảnh báo** khi: khoảng cách ngắn hơn tối thiểu; tiêm sau tuổi tối đa; số mũi user nhập mâu thuẫn với thứ tự thời gian. Hệ quả trên gợi ý xem F05.10.

**F04.5 "Tôi không nhớ lịch sử tiêm thời nhỏ"**
- Một lần chạm khi tạo hồ sơ (hoặc trong cài đặt hồ sơ): các mốc của mọi phác đồ có thuộc tính **giai đoạn = thơ ấu** (thuộc tính của phác đồ, admin nhập) có ngày đến hạn **trước thời điểm chọn lựa chọn này** được coi là "chưa rõ". Các mốc sau được tính bình thường — cùng mốc cắt với lựa chọn 1 của F03.6 (để các mốc tương lai của trẻ nhỏ không bị kẹt ở ⚪).
- Cờ này chỉ là **mặc định**: bệnh nào đã có mũi tiêm hoặc khai báo theo bệnh riêng thì dữ liệu riêng được ưu tiên (F04.6). Các bệnh chắc chắn đã quá tuổi thì vào khối "Không còn trong độ tuổi tiêm" (F05.11).
- Các mục này gom vào **một nhóm "Nên hỏi bác sĩ hoặc xét nghiệm"** (ví dụ xét nghiệm kháng thể viêm gan B), không hiển thị thành từng mục đỏ "quá hạn".
- Gợi ý ưu tiên những việc làm được ngay ở tuổi hiện tại (cúm hằng năm, nhắc uốn ván – bạch hầu, HPV nếu trong độ tuổi, zona / phế cầu theo tuổi, MMR & thuỷ đậu trước khi mang thai…).

**F04.6 Ghi nhận theo bệnh (sau khi hỏi bác sĩ)**

Khai báo theo bệnh có **hai loại**, với thứ tự ưu tiên khác nhau:
- **Khai báo về lịch sử** — "chưa rõ" (F04.2 mức 5) và "đã xác nhận chưa tiêm" (nút nhanh F03.6). Chúng mô tả lịch sử tiêm: mũi tiêm của bệnh được ưu tiên hơn chúng, còn chúng được ưu tiên hơn tình trạng lịch sử của hồ sơ (F03.6, F04.5).
- **Khai báo ghi nhận** — bốn ghi nhận trong bảng dưới. Khi còn hiệu lực, chúng **thắng cả khi đã có mũi tiêm** và chuyển bệnh vào khối "Đã ghi nhận" (F05.11 tầng 2).

| Ghi nhận | Hệ quả |
|---|---|
| Đã có miễn dịch (xét nghiệm / đã mắc) | Chuyển vào khối "Đã ghi nhận"; **không chọn được** cho bệnh có nhắc lại định kỳ (uốn ván, cúm…), kể cả khi đã có mũi tiêm |
| Bác sĩ khuyên không tiêm | Chuyển vào khối "Đã ghi nhận" |
| Hoãn đến [ngày] | Chuyển vào khối "Đã ghi nhận"; tới ngày đó ghi nhận hết hiệu lực và mục **tự quay lại** nhóm do bộ máy tính, kèm banner trong app (F08.1) |
| Tôi chọn không tiêm lúc này | Chuyển vào khối "Đã ghi nhận"; **hỏi lại sau 12 tháng** (qua trung tâm thông báo). Mục ở lại khối cho tới khi user trả lời lần hỏi lại; không tự quay lại |
- Mục đã ghi nhận không gửi email, không tính vào mức độ cần chú ý.
- Huỷ ghi nhận được bất cứ lúc nào → mục quay lại nhóm theo bộ máy gợi ý.
- Cả hai loại được lưu thành khai báo theo bệnh của hồ sơ (DiseaseDeclaration, §5), kèm loại và ngày tạo khai báo.

### F05 — Bộ máy gợi ý

**F05.1 Đầu vào**
Tuổi (từ ngày sinh với độ chính xác tương ứng), giới tính, lịch sử tiêm (kèm độ chính xác), sản phẩm đã dùng, khai báo theo bệnh của hồ sơ, cờ mang thai / dự định mang thai, tình trạng lịch sử của hồ sơ và thời điểm chọn (F03.6), **phiên bản bộ quy tắc** đang xuất bản.
- **Mũi kế tiếp** được tính theo quy tắc và nhãn TCMR **có hiệu lực tại ngày tính toán** (hôm nay). Mũi đã ghi được kiểm tra theo quy tắc có hiệu lực tại ngày tiêm **hoặc** quy tắc có hiệu lực hôm nay (F05.10).
- Bệnh được tính lịch mà **không có phác đồ nào có hiệu lực** tại ngày tính toán → ⚪ `NO_SERIES_IN_EFFECT`; không bao giờ bị bỏ qua lặng lẽ.
- **Kiểm tra dữ liệu đầu vào** là một bước tách khỏi việc tính toán; server và việc nhập dữ liệu khách chạy bước này trước khi tính. Bộ máy không bao giờ tự ép kiểu giá trị: ngày tiêm không đọc được → ⚪ `INVALID_DATE` cho các bệnh của bản ghi đó; ngày sinh không hợp lệ, hoặc tình trạng lịch sử "Chưa nhập xong" mà không có thời điểm chọn, là lỗi đầu vào và không tạo đánh giá nào.

**F05.2 Các loại quy tắc cần hỗ trợ** (rút ra từ sổ VNVC và nghiên cứu)
| Loại | Ví dụ |
|---|---|
| Phác đồ chuẩn theo mốc tuổi | Viêm gan B sơ sinh trong 24h; 6in1 lúc 2-3-4 tháng |
| **Phác đồ thay thế** cho cùng bệnh (các phương án thay thế trong một phác đồ; mọi phương án phải có cùng số mũi) | 2-3-4 tháng hoặc 2-4-6 tháng (các ô có dấu `*` trong sổ) |
| **Phác đồ theo sản phẩm × tuổi bắt đầu** (bắt buộc có tuổi bắt đầu khi một cặp (bệnh, sản phẩm) có nhiều hơn một phác đồ) | Rotarix 2 liều / RotaTeq 3 liều; Gardasil 9: 9–14 tuổi 2 mũi, 15–45 tuổi 3 mũi |
| Tuổi tối thiểu / tuổi tối đa bắt đầu / tuổi tối đa hoàn thành | Rota phải xong trước 24 hoặc 32 tuần tuỳ sản phẩm; Hib không cần sau 5 tuổi |
| Khoảng cách tối thiểu giữa các mũi (tính từ mũi trước, và tuỳ chọn tính từ mũi đầu) | MMR 2 mũi cách ≥ 1 tháng |
| **Tiêm bù** ("nếu chưa tiêm") theo tuổi | Viêm gan A 2 mũi nếu chưa tiêm. Một phác đồ có thể đặt **tuổi tiêm bù** tuỳ chọn (`catchUpAfterAge`): khi mũi kế tiếp rơi vào sau tuổi đó → ⚪ `CATCH_UP_ASK_DOCTOR` "lịch tiêm bù — hỏi bác sĩ" thay cho 🟠/🔴. Số mũi giảm theo tuổi lúc tiêm mũi hiện tại chưa được mô hình hoá trong MVP |
| **Nhắc lại định kỳ** | Cúm mỗi năm; uốn ván 5–10 năm; viêm não Nhật Bản, thương hàn 3 năm. Mỗi quy tắc ghi rõ khoảng nhắc tính theo "số tháng từ mũi trước" hay "theo mùa", và hành vi khi mũi cuối không rõ — ở mức 4, hoặc phác đồ là ⚪ do lịch sử mà không có mũi nào có ngày ("coi như đến hạn" hoặc "⚪ hỏi bác sĩ"), admin nhập kèm nguồn; không có nguồn thì mặc định ⚪. Cách tính: F05.12 |
| **Mũi bổ sung được phép** | Viêm gan B sơ sinh đơn giá không làm đổi phác đồ 6in1 |
| **Yêu cầu cùng sản phẩm** (quy tắc đặc biệt cấp bệnh, kèm trích dẫn) | Rota: nên hoàn tất bằng cùng một sản phẩm (F04.3) |
| **Trước mang thai** | MMR, thuỷ đậu nên tiêm trước khi mang thai ít nhất 3 tháng |
| **Chỉ thông tin** (không tính lịch) | Dại (theo phơi nhiễm / nguy cơ), tả |

- Phác đồ đang dở: **tiếp tục, không bắt đầu lại** (theo WHO Table 3 — xem tài liệu nghiên cứu).
- Các thông số dung sai, khoảng đệm quá hạn… là **trường của quy tắc**, admin nhập kèm nguồn; không có nguồn thì mặc định 0.
- **Hằng số hiển thị** — 7 ngày ân hạn trước khi quá hạn (F05.4), ngưỡng 30 ngày và 6 tháng của các nhóm (F05.4), và ghi chú "sau sinh" kéo dài 3 tháng (F05.7) — là hằng số của bộ máy, không phải quy tắc y khoa; yêu cầu trích nguồn của P2 không áp dụng cho chúng.
- **Kiểm tra bộ quy tắc** (chạy trên mọi changeset): mọi phương án thay thế trong một phác đồ có cùng số mũi — các phương án có số mũi khác nhau chỉ được nhập thành các phác đồ riêng, khác nhau về sản phẩm hoặc về khoảng tuổi bắt đầu không chồng lấn; nếu không, admin chọn một phương án (kèm trích dẫn) hoặc bệnh không được tính lịch. Có tuổi bắt đầu khi một cặp (bệnh, sản phẩm) có nhiều hơn một phác đồ. Hiện cảnh báo khi một changeset kết thúc phác đồ cuối cùng của một bệnh được tính lịch.

**F05.3 Gợi ý theo bệnh hay theo sản phẩm (lai)**
- Phác đồ được tính **theo từng bệnh**; mỗi mũi phối hợp được tính là một mũi cho từng bệnh mà nó phòng. Phác đồ của sản phẩm X chỉ quyết định các thuộc tính riêng của X (số mũi, khoảng cách, tuổi tối đa).
- Chưa tiêm mũi nào → gợi ý theo **bệnh**, liệt kê các sản phẩm phù hợp với tuổi (với hồ sơ đang mang thai, không gồm vaccine sống — F05.7). Khi ngày sinh là một khoảng, mỗi đầu khoảng dùng nhóm tuổi bắt đầu có hiệu lực tại đầu đó (F05.5).
- Đã tiêm mũi đầu bằng sản phẩm X → các mũi sau tính theo **thuộc tính phác đồ của X**.
- **"Phác đồ an toàn nhất"** — áp dụng khi các mũi của một bệnh dùng sản phẩm khác nhau, hoặc có mũi không rõ sản phẩm / sản phẩm ngoài danh mục. **Các sản phẩm khả dĩ** là các sản phẩm trong danh mục, kể cả sản phẩm đã ngừng (xoá mềm) — chúng vẫn nằm trong phiên bản bộ quy tắc cùng các phác đồ của mình và được tính như không rõ sản phẩm (F11.3). Lấy theo từng chiều trong các phác đồ ứng viên: số mũi = **lớn nhất**; khoảng cách tối thiểu = **dài nhất**; tuổi tối thiểu = **muộn nhất**; tuổi tối đa = **L nhỏ nhất** (tuổi tối đa hoàn thành − 1 ngày, tính ở đầu già của khoảng ngày sinh) trong các ứng viên còn cần mũi đó, từ đó suy ra hạn chót (F05.4b).
- **Tuổi tối đa không thống nhất**: nếu các ứng viên có tuổi tối đa khác nhau và cho kết luận khác nhau → ⚪ Chưa rõ, không báo quá tuổi (P1). Lý do tuỳ theo sản phẩm của các mũi:
  - có mũi không rõ sản phẩm → `CHECK_PRODUCT_NAME` "Kiểm tra tên vaccine trong sổ tiêm";
  - mọi mũi đều có tên sản phẩm (trong danh mục, đã ngừng, hoặc tên tự do) → `MIXED_PRODUCTS` "đã dùng nhiều loại vaccine — hỏi bác sĩ".
- **Sản phẩm ngoài danh mục**: khi phác đồ đã đủ theo phác đồ an toàn nhất trên các sản phẩm trong danh mục → ✅ kèm nhãn ước lượng và "có mũi ngoài danh mục — xác nhận với bác sĩ đã đủ mũi" (lý do `PRODUCT_NOT_IN_CATALOG`).
- **Thứ tự gộp** các ngày: (1) trong từng phác đồ ứng viên, gộp các phương án thay thế (F05.4); (2) giữa các phác đồ ứng viên, ngày sớm nhất được phép, ngày đến hạn và ngày quá hạn mỗi ngày lấy giá trị **muộn nhất**; (3) sau đó áp dụng giới hạn tuổi tối đa (F05.4b). Ghi chú "có thể theo lịch …" được giữ nếu có bất kỳ phác đồ ứng viên nào có phương án thay thế.
- Phần giải thích phác đồ an toàn nhất liệt kê từng phác đồ ứng viên kèm sản phẩm của nó, hoặc "generic" (chung) với phác đồ không gắn sản phẩm.

**F05.4 Nhóm kết quả**

Định nghĩa ngày cho mỗi mũi kế tiếp:
- **Ngày sớm nhất được phép** = ngày thoả tuổi tối thiểu và các khoảng cách tối thiểu.
- **Ngày đến hạn** = max(ngày sớm nhất được phép, mốc khuyến cáo), sau đó không vượt quá hạn chót khi mũi vẫn còn tiêm kịp, tức max(hôm nay, ngày sớm nhất được phép) không sau hạn chót (F05.4b).
- **Ngày quá hạn** = ngày đến hạn + **max(khoảng đệm, 7 ngày)**; khoảng đệm là trường của quy tắc (mặc định 0). 7 ngày là hằng số hiển thị của bộ máy (F05.2), khớp với email "Bạn đã tiêm chưa?" vào ngày thứ 7 sau ngày đến hạn (F08); khoảng đệm có nguồn dài hơn 7 ngày vẫn được ưu tiên. Mũi bị giới hạn bởi tuổi tối đa thì ngày quá hạn cũng bị giới hạn (F05.4b). Mũi nhắc lại theo mùa dùng ngày cuối của khung mùa thay thế (F05.12).

| Nhóm | Điều kiện |
|---|---|
| 🔴 Quá hạn / cần tiêm bù | Hôm nay ≥ ngày quá hạn **và chắc chắn** còn trong tuổi được tiêm. **Không bao giờ** áp dụng cho mũi bị giới hạn bởi tuổi tối đa (F05.4b), mũi nhắc lại theo mùa, hoặc mũi nhắc lại hiện là đến hạn sau một mũi cuối không rõ (F05.12) |
| 🟠 Đến hạn | Hôm nay ≥ ngày sớm nhất được phép, ngày đến hạn ≤ hôm nay + 30 ngày, và hôm nay < ngày quá hạn. Vì vậy đúng ngày đến hạn mũi là 🟠, và từ ngày đến hạn + 7 ngày (với khoảng đệm ≤ 7) là 🔴. Mũi bị giới hạn bởi tuổi tối đa giữ 🟠 tới hạn chót, kèm "hạn chót DD/MM" (F05.4b) |
| 🟡 Sắp tới | Ngày đến hạn trong (hôm nay + 30 ngày, hôm nay + 6 tháng], **hoặc** hôm nay < ngày sớm nhất được phép và ngày đến hạn ≤ hôm nay + 30 ngày |
| ⚪ Chưa rõ | Thiếu dữ liệu để kết luận, ví dụ: hai đầu khoảng ngày sinh cho kết quả khác nhau theo cách có ý nghĩa (F05.5); các sản phẩm ứng viên không thống nhất (F05.3); mũi có thể không hợp lệ, trừ khi khối quá tuổi thắng (F05.10); mũi gần nhất ở mức 4 và bệnh không chắc chắn đã quá tuổi (F04.2); không có ngày sinh khi giới hạn tuổi có ý nghĩa (F05.5). Lý do nào hiển thị, và trạng thái nào bị che, theo F05.11. Có nhóm con "Nên hỏi bác sĩ hoặc xét nghiệm" |
| ✅ Đã hoàn thành | Đủ phác đồ (và chưa đến kỳ nhắc), kể cả phác đồ đủ theo các sản phẩm trong danh mục khi có mũi dùng sản phẩm ngoài danh mục (nhãn ước lượng, F05.3). Phác đồ đang theo **đã có ít nhất một mũi** mà mũi kế tiếp còn **hơn 6 tháng** cũng nằm ở đây, với nhãn "Đang theo phác đồ — mũi tiếp dự kiến MM/YYYY" |

Khi **chưa tiêm mũi nào** và ngày đến hạn của mũi 1 còn hơn 6 tháng sau hôm nay, mũi đó vào khối "Chưa tới lịch tiêm" (F05.4a), không vào ✅; theo thời gian nó chuyển sang 🟡 rồi 🟠.

Mỗi mũi kế tiếp thuộc **đúng một** nhóm ở bảng trên hoặc một khối ở F05.4a; khi nhiều trạng thái cùng áp dụng, thứ tự ưu tiên ở F05.11 quyết định.

**F05.4a Khối ngoài 5 nhóm** (thu gọn, ở cuối trang, **không tô đỏ**, không email, không tính mức độ cần chú ý)
| Khối | Nội dung |
|---|---|
| Chưa tới lịch tiêm | Chưa tiêm mũi nào và ngày đến hạn của mũi 1 còn **hơn 6 tháng** sau hôm nay. Nhãn mục: "Chưa tới tuổi tiêm — dự kiến MM/YYYY" khi hôm nay < ngày sớm nhất được phép; ngược lại "Theo lịch khuyến cáo — dự kiến MM/YYYY, có thể tiêm từ MM/YYYY". Hồ sơ đang mang thai: xem F05.7 |
| Không còn trong độ tuổi tiêm — hỏi bác sĩ | Quá tuổi tối đa bắt đầu, hoặc phác đồ dang dở đã quá tuổi tối đa hoàn thành, **hoặc mũi kế tiếp không còn tiêm kịp trước tuổi tối đa** (max(hôm nay, ngày sớm nhất được phép) sau hạn chót, F05.4b) (ví dụ Rota ở bé 7 tháng; Hib sau 5 tuổi) — chỉ khi **chắc chắn** quá tuổi, tức là ở đầu trẻ của khoảng ngày sinh (P1) |
| Chỉ lưu trữ — không nhắc lịch | Mũi đã ghi cho bệnh chỉ thông tin / không tính lịch (ví dụ dại) |
| Đã ghi nhận | Các mục có khai báo ghi nhận đang hiệu lực (F04.6) |
| Hoãn trong thai kỳ / Trong thai kỳ | Xem F05.7 |

Các trạng thái "Không còn trong độ tuổi tiêm" **dựa trên tuổi** — quá tuổi tối đa bắt đầu hoặc hoàn thành — được ánh xạ sang ô **"Không khuyến cáo"** trong bảng theo độ tuổi (F06) để bảng và gợi ý không lệch nhau. Trạng thái còn phụ thuộc vào ngày tiêm của chính hồ sơ (mũi kế tiếp không còn kịp trước tuổi tối đa) không có ô tương ứng trong bảng, vì bảng không biết lịch sử tiêm.

**Phác đồ thay thế** (các phương án thay thế trong một phác đồ, ví dụ 2-3-4 hay 2-4-6 tháng; mọi phương án có cùng số mũi, F05.2): mỗi phương án được tính riêng. Ngày đến hạn — và ngày sớm nhất được phép lấy từ cùng phương án đó — theo phương án cho **ngày đến hạn sớm nhất**; ngày quá hạn theo phương án **muộn nhất**. Gộp giữa các phác đồ ứng viên và giới hạn tuổi tối đa: thứ tự gộp ở F05.3. Thẻ ghi "có thể theo lịch 2-3-4 hoặc 2-4-6 tháng — hỏi bác sĩ"; không tự suy phương án từ khoảng cách đã tiêm. Mũi đã ghi là hợp lệ nếu thoả quy tắc mũi của ít nhất một phương án (F05.10).

Mỗi gợi ý hiển thị:
- Nhãn **TCMR (miễn phí tại trạm y tế)** hoặc **Dịch vụ (có phí)** theo **từng sản phẩm** liệt kê (xem F07). Với vaccine TCMR đang triển khai dần (Rota, phế cầu, HPV): ghi *"TCMR — có thể chưa triển khai tại địa phương, hỏi trạm y tế"*. Không hỏi tỉnh / thành của user.
- Nguồn trích dẫn + ngày cập nhật bộ quy tắc.
- Dòng miễn trừ chuẩn (F10.1 tầng 2).
- Nhãn "⚠️ dựa trên thông tin ước lượng" khi áp dụng.
- Dòng hạn chót "hạn chót DD/MM" khi ngày đến hạn bị giới hạn (F05.4b).

**F05.4b Giới hạn tuổi tối đa (hạn chót)**

Với phác đồ có tuổi tối đa hoàn thành, bộ máy tính một **hạn chót** cho từng mũi kế tiếp, để, mỗi khi ngày sớm nhất được phép cho phép, ngày đến hạn không bao giờ rơi vào sau ngày cuối cùng mũi đó còn tiêm được mà vẫn kịp hoàn thành phác đồ:
- **L** = tuổi tối đa hoàn thành − 1 ngày, tính ở **đầu già** của khoảng ngày sinh (P1).
- **L_k** (hạn chót của mũi k) = L trừ tổng các khoảng cách tối thiểu của các mũi sau mũi k — ngày cuối cùng có thể tiêm mũi k mà vẫn còn đủ thời gian hoàn thành phác đồ. Khi chưa tiêm mũi nào, lấy thêm min với (tuổi tối đa bắt đầu − 1 ngày).
- Khi có nhiều phác đồ ứng viên (không rõ sản phẩm, nhiều loại sản phẩm, sản phẩm ngoài danh mục), dùng **L nhỏ nhất** trong các ứng viên còn cần mũi k. Giới hạn được áp dụng **sau** bước gộp ở F05.3.

Sau đó, với mũi kế tiếp k, đặt **F** = max(hôm nay, ngày sớm nhất được phép) (ngày đầu tiên kể từ hôm nay có thể tiêm mũi này):
| Trường hợp | Kết quả |
|---|---|
| F ≤ L_k | Ngày đến hạn và ngày quá hạn = min(giá trị hiện tại, L_k), bất kể khoảng đệm lấy từ nguồn nào. Nếu việc này làm sớm một trong hai ngày, mũi **bị giới hạn**: lý do `DUE_CAPPED_BY_MAX_AGE`, **không bao giờ 🔴**, hiện 🟠 tới L_k kèm "hạn chót DD/MM"; các email nhắc trước ngày đến hạn (7 ngày, 1 ngày) dùng ngày đến hạn đã giới hạn (F08). Nếu không, các ngày giữ nguyên và nhóm theo F05.4 |
| L_k < F ≤ L | 🟠 tới L, kèm "có thể không kịp đủ phác đồ — hỏi bác sĩ" (lý do `SERIES_MAY_NOT_COMPLETE`, chỉ mang tính thông tin: các mũi vẫn được tiêm tới tuổi tối đa). Chuỗi các mũi còn lại được tính từ F, ở đầu trẻ của khoảng ngày sinh |
| F > L | Khối "Không còn trong độ tuổi tiêm" khi điều này chắc chắn ở đầu trẻ của khoảng ngày sinh; ngược lại ⚪ kèm "Nhập ngày sinh đầy đủ" (lý do `BIRTH_DATE_IMPRECISE`) |

**F05.5 Ngày sinh không đầy đủ**
- **Tháng/năm hoặc chỉ năm** → tuổi là một *khoảng*; bộ máy xét cả hai đầu (đầu muộn hơn không vượt quá hôm nay). Quy tắc cho cùng kết quả ở hai đầu khoảng → gợi ý bình thường.
  - Khác biệt chỉ làm đổi **khoảng cách còn bao xa** tới một mốc — giữa khối "Chưa tới lịch tiêm", 🟡 và ✅ "Đang theo phác đồ" — **không** được coi là kết quả khác nhau: hiển thị kết quả của đầu muộn hơn kèm nhãn ước lượng.
  - Mọi khác biệt liên quan tới 🟠, 🔴, quá tuổi, giới hạn tuổi tối đa hoặc tính hợp lệ của mũi → ⚪ Chưa rõ, kèm "Nhập tháng sinh để gợi ý chính xác hơn".
  - Các ngày do tuổi tối thiểu hoặc khoảng cách quyết định lấy từ đầu **muộn hơn** (không bao giờ nhắc trước tuổi tối thiểu); các ngày bị giới hạn bởi tuổi tối đa lấy từ đầu **già** (F05.4b).
- **Tuổi tối đa**: nếu ở đầu già của khoảng tuổi đã có thể quá tuổi nhưng đầu trẻ chưa chắc chắn quá tuổi → ⚪ kèm "Nhập ngày sinh đầy đủ".
- **Nhóm tuổi bắt đầu khi chưa tiêm mũi nào**: mỗi đầu khoảng ngày sinh dùng nhóm tuổi bắt đầu (phác đồ) có hiệu lực tại đầu đó, và hai kết quả được so sánh như trên.
- **Tuổi lúc tiêm** khi cả ngày sinh và ngày tiêm là khoảng: nếu hai đầu khoảng cho phác đồ khác nhau (ví dụ Gardasil 9: 14 vs 15 tuổi) → dùng phác đồ **nhiều mũi hơn** và gắn nhãn ước lượng. Quy tắc này chỉ áp dụng khi đã có mũi tiêm (nó nói về tuổi lúc tiêm một mũi đã ghi).
- **Không có ngày sinh** → không tính gợi ý theo tuổi cho phác đồ chưa tiêm mũi nào. Với phác đồ đang dở, chỉ các mốc khuyến cáo tính từ ngày sinh mới được thay thế; các mốc tính từ mũi đầu hoặc mũi trước được tính bình thường, cũng như các mũi nhắc lại định kỳ. Hiển thị bảng theo độ tuổi để user tự đối chiếu.
  - Mốc tính từ ngày sinh của mũi k: ngày đến hạn = max(ngày sớm nhất được phép, mũi trước + (mốc của mũi k − mốc của mũi k−1)), tức là giữ nguyên khoảng cách khuyến cáo giữa các mốc (ví dụ sởi mũi 2 = mũi 1 + 9 tháng, không phải + 1 tháng); gắn nhãn ước lượng.
  - Bỏ qua kiểm tra mũi theo tuổi (tuổi tối thiểu, tuổi tối đa bắt đầu, tuổi tối đa hoàn thành); vẫn giữ kiểm tra khoảng cách.
  - Mũi kế tiếp là ⚪ (lý do `NO_BIRTH_DATE`) nếu nó có tuổi tối thiểu không suy ra được từ tuổi tối thiểu của mũi 1 cộng các khoảng cách tối thiểu, hoặc nếu nó có tuổi tối đa hoàn thành. Tuổi tối đa bắt đầu chỉ áp dụng cho mũi 1.
  - Kiểm tra bộ quy tắc có thể xác nhận tuổi tối thiểu của một mũi ≤ tuổi tối thiểu của mũi 1 + tổng các khoảng cách tối thiểu ở giữa.

**F05.6 Giới tính để trống**
- Vẫn hiện gợi ý phụ thuộc giới tính, kèm ghi chú (ví dụ "áp dụng cho nữ").
- Cờ mang thai bật được khi giới tính là Nữ hoặc để trống; **ẩn khi giới tính là Nam**.

**F05.7 Cờ mang thai / dự định mang thai**
- **Đang mang thai**: nhập ngày dự sinh, trong khoảng (hôm nay, hôm nay + 10 tháng]; cờ tự tắt sau ngày dự sinh.
- **Dự định mang thai**: nhắc user xác nhận lại sau 6 tháng — **chỉ qua trung tâm thông báo trong app**, không qua email; nếu user chưa xác nhận thì cờ giữ nguyên và lời nhắc vẫn hiện.
- Sản phẩm có thuộc tính **vaccine sống** (admin nhập kèm nguồn). Khi "Đang mang thai" bật:
  - Các sản phẩm vaccine sống bị loại khỏi danh sách sản phẩm của mục. Nếu không còn sản phẩm nào và mục đang ở 🔴/🟠/🟡, mục chuyển sang khối **"Hoãn trong thai kỳ — hỏi bác sĩ"** và **không gửi email nhắc** (F05.11 tầng 8). Nếu còn sản phẩm không phải vaccine sống, mục ở nguyên nhóm và chỉ liệt kê các sản phẩm đó.
  - Mục vaccine sống trong khối "Chưa tới lịch tiêm" hoặc ở ✅ "Đang theo phác đồ" có tháng dự kiến trước hoặc đúng ngày dự sinh thì hiện "sau sinh — hỏi bác sĩ" thay cho tháng.
  - Nếu có vaccine sống được ghi trong 3 tháng trước khi bật cờ → hiện "hỏi bác sĩ" trên bệnh đó (lý do `LIVE_VACCINE_BEFORE_PREGNANCY`).
  - Khi cờ tự tắt, các mục này quay lại nhóm thường kèm ghi chú "sau sinh — hỏi bác sĩ". Ghi chú kéo dài **3 tháng sau ngày dự sinh**, hoặc tới khi có mũi tiêm của bệnh đó được ghi; 3 tháng là hằng số hiển thị (F05.2), không phải giá trị y khoa.
- Vaccine thường được cân nhắc trong thai kỳ (uốn ván, ho gà, cúm, RSV) hiển thị trong khối riêng **"Trong thai kỳ — hỏi bác sĩ về thời điểm"**: không xếp vào 5 nhóm, không có ngày đến hạn, không gửi email; nhãn TCMR theo nhãn của sản phẩm (F05.11 tầng 4).
- Quy tắc "trước mang thai" (MMR, thuỷ đậu ≥ 3 tháng) áp dụng khi cờ "Dự định mang thai" bật. Quy tắc này chỉ thêm ghi chú "nên tiêm xong ít nhất 3 tháng trước khi mang thai" vào đánh giá bình thường của bệnh; tự nó không bao giờ tạo ra 🔴. Email không bao giờ nhắc tới mang thai hay dự định mang thai (F08).
- **Không** đưa mốc tuần thai cụ thể cho đến khi có nguồn chính thức đã kiểm chứng.

**F05.8 Phạm vi bệnh được tính lịch**
- **TCMR** theo TT 10/2024/TT-BYT và TT 13/2026/TT-BYT: 11 bệnh của TT 10/2024 cộng **phế cầu** và **HPV**. Mục "bệnh khác do Bộ trưởng quy định" không tính lịch.
- Phế cầu, HPV, Rota được tính lịch với **cả sản phẩm TCMR lẫn dịch vụ**.
- Vaccine dịch vụ phổ biến: cúm, HPV, phế cầu, Rota, MMR, thuỷ đậu, viêm gan A, não mô cầu, sốt xuất huyết (Qdenga), zona (Shingrix).
- **RSV**: chỉ thông tin trong thai kỳ (F05.7), không tính lịch.
- Bệnh / vaccine khác vẫn có trong danh mục nhưng **không tính lịch** → "Tham khảo bác sĩ".

**F05.9 Tính lại**
- Gợi ý được tính lại khi: hồ sơ hoặc lịch sử thay đổi; một phiên bản bộ quy tắc mới được xuất bản hoặc rollback; theo thời gian (tuổi tăng, mốc đến hạn).

**F05.10 Mũi không hợp lệ**
- **Quá sớm**: mũi **mức 1** vi phạm tuổi tối thiểu, khoảng cách tối thiểu từ mũi trước hoặc khoảng cách tối thiểu từ mũi đầu → **không** được tính là hoàn thành mũi đó; bệnh xếp ⚪ kèm "Mũi này có thể sớm hơn khuyến cáo — hỏi bác sĩ có cần tiêm lại không".
- **Sau tuổi tối đa**: mũi 1 chắc chắn tiêm sau tuổi tối đa bắt đầu, hoặc bất kỳ mũi nào chắc chắn tiêm sau tuổi tối đa hoàn thành → ⚪ kèm "mũi này tiêm sau độ tuổi khuyến cáo — hỏi bác sĩ" (lý do `DOSE_AFTER_MAX_AGE`); mũi đó không được tính.
- Mũi **mức 2–3** được kiểm tra ở cả hai đầu khoảng ngày đã giới hạn tới hôm nay [đầu kỳ, min(cuối kỳ, hôm nay)] (F04.2); chỉ vi phạm ở một đầu → ⚪ (lý do `DOSE_POSSIBLY_INVALID`).
- **Nhiều phác đồ ứng viên** (F05.3): việc kiểm tra chạy theo từng ứng viên. Chỉ ra `DOSE_AFTER_MAX_AGE` khi mọi ứng viên đều thống nhất; nếu không thì ⚪ `CHECK_PRODUCT_NAME` (có sản phẩm không rõ) hoặc `MIXED_PRODUCTS` (mọi sản phẩm đều có tên). Khi các ứng viên không thống nhất, bộ máy không bao giờ kết luận "chưa tiêm mũi nào" hay "quá tuổi" từ việc một mũi không được tính.
- **Mũi sau một mũi mức 4 hoặc mũi ngầm định**: không kiểm tra được khoảng cách tối thiểu tới mũi trước không có ngày, nên bỏ qua khoảng cách này — mũi được tin là hợp lệ (được tính), kết quả gắn nhãn ước lượng kèm "không kiểm tra được khoảng cách — đối chiếu sổ tiêm" (lý do `INTERVAL_UNVERIFIABLE`), và bệnh có thể đạt ✅. Chỉ bỏ qua đúng khoảng cách đó: tuổi tối thiểu, các tuổi tối đa và khoảng cách tối thiểu từ mũi đầu luôn được kiểm tra khi có đủ ngày cần thiết. Nếu không có cách đặt (các) mũi không có ngày nào thoả mọi ràng buộc (với các mũi có ngày kề bên và khoảng ngày sinh) → ⚪ (lý do `DOSE_INVALID`). Khoảng cách tối thiểu của mũi j không bao giờ được kiểm tra so với mũi j−2.
- **Phương án thay thế**: mũi đã ghi là hợp lệ nếu thoả quy tắc mũi của ít nhất một phương án thay thế; không bao giờ suy ra phương án từ mũi đó.
- **Phiên bản quy tắc**: mũi đã ghi là hợp lệ nếu thoả quy tắc có hiệu lực tại ngày tiêm **hoặc** quy tắc có hiệu lực hôm nay; mũi kế tiếp dùng quy tắc có hiệu lực hôm nay (F05.1).
- **Thứ tự ưu tiên**: nếu người được tiêm chắc chắn đã quá tuổi và phác đồ vẫn chưa đủ kể cả khi tính mọi mũi đang bị nghi vấn, khối "Không còn trong độ tuổi tiêm" thắng các kết quả ⚪ này; nếu tính mũi bị nghi vấn sẽ làm đủ phác đồ thì giữ ⚪ (P1) (F05.11 tầng 5).
- **Dung sai** (nếu có) là trường của quy tắc, admin nhập kèm nguồn, mặc định 0. Dung sai **chỉ dùng khi kiểm tra các mũi đã tiêm**, áp dụng cho tuổi tối thiểu, khoảng cách tối thiểu và khoảng cách tối thiểu từ mũi đầu, **không bao giờ** cho các tuổi tối đa, và không bao giờ được dùng để tính ngày sớm nhất được phép hay ngày đến hạn (P1). Dung sai được kiểm tra ở cả hai đầu khoảng ngày (đã giới hạn tới hôm nay).

**F05.11 Thứ tự ưu tiên**

Khi nhiều trạng thái cùng áp dụng cho một mũi kế tiếp, tầng đầu tiên áp dụng (từ cao xuống thấp) quyết định nhóm hoặc khối. Lý do từ các tầng bị che vẫn được giữ kèm đánh giá.
1. Bệnh chỉ thông tin → khối "Chỉ lưu trữ".
2. Khai báo ghi nhận đang hiệu lực (F04.6: đã có miễn dịch, bác sĩ khuyên không tiêm, hoãn đến ngày, chọn không tiêm lúc này) → khối "Đã ghi nhận".
3. Tạm ngưng bệnh, phác đồ hoặc quy tắc (F11.8) → ⚪ "Tạm ngưng tính lịch — tham khảo bác sĩ".
4. Đang mang thai và bệnh nằm trong danh sách "được cân nhắc trong thai kỳ" → khối "Trong thai kỳ" (F05.7).
5. **Chắc chắn đã quá tuổi** (F05.4a, xét ở đầu trẻ của khoảng ngày sinh; với mũi trước ở mức 4: hôm nay ≥ tuổi tối đa hoàn thành, bất kể ngày của các mũi trước) → khối "Không còn trong độ tuổi tiêm". Tầng này thắng các mặc định theo tình trạng lịch sử (lựa chọn 1 kể cả nút nhanh cho người lớn, lựa chọn 2), thắng mũi trước ở mức 4, và thắng tính hợp lệ của mũi khi phác đồ vẫn chưa đủ kể cả khi tính mọi mũi đang bị nghi vấn (nếu tính mũi bị nghi vấn sẽ làm đủ phác đồ thì giữ ⚪ — P1). Lý do: F04.5 ưu tiên những việc làm được ngay, và hỏi người 45 tuổi về Rota hay Hib chỉ gây nhiễu (P5). Khối này thu gọn và không bao giờ tô đỏ.
6. Mặc định theo tình trạng lịch sử: ⚪ "Chưa nhập" (F03.6 lựa chọn 1, kể cả nút nhanh cho người lớn "Bạn đã từng tiêm chưa?"), ⚪ "Nên hỏi bác sĩ hoặc xét nghiệm" (lựa chọn 2, F04.5).
7. ⚪ do dữ liệu: khai báo về lịch sử "chưa rõ" (mức 5), tính hợp lệ của mũi (F05.10), mũi trước ở mức 4 (F04.2), hai đầu của ngày sinh không đầy đủ cho kết quả khác nhau (F05.5), không chắc chắn về sản phẩm (F05.3), không có ngày sinh (F05.5).
8. Nhóm tính được (F05.4, F05.4b) hoặc khối "Chưa tới lịch tiêm". Sau đó, với hồ sơ đang mang thai, các mục vaccine sống ở 🔴/🟠/🟡 chuyển sang khối "Hoãn trong thai kỳ" (F05.7).

Ghi chú: việc đặt tầng 5 trên tầng 6 đảo ngược thứ tự trước đây, khi "Chưa nhập" đứng trước bước kiểm tra tuổi tối đa.

**F05.12 Nhắc lại định kỳ**
- Mũi nhắc lại định kỳ bắt đầu sau khi phác đồ cơ bản đã đủ; số thứ tự mũi tiếp nối số thứ tự của phác đồ.
- Bệnh chỉ có nhắc lại định kỳ (ví dụ cúm ở người lớn) được mô hình hoá thành một phác đồ một mũi cộng một quy tắc nhắc lại.
- Nhắc lại **"mỗi N tháng"**: ngày sớm nhất được phép = mũi cuối + khoảng cách tối thiểu; ngày đến hạn = mũi cuối + N tháng; ngày quá hạn = ngày đến hạn + max(khoảng đệm, 7 ngày) (F05.4).
- Nhắc lại **"theo mùa"**: ngày đến hạn = ngày bắt đầu khung mùa; ngày quá hạn = ngày kết thúc khung mùa (không có 7 ngày ân hạn). Nếu hết khung mùa mà chưa tiêm, mũi kế tiếp chuyển sang khung của mùa sau (🟡 / ✅ / khối); **không bao giờ 🔴**.
- **Mũi cuối không rõ** — mũi cuối ở mức 4, hoặc phác đồ cơ bản là ⚪ do lịch sử (lựa chọn 2, lựa chọn 1 "Chưa nhập", mức 5) mà không có mũi nào có ngày: theo hành vi của quy tắc khi mũi cuối không rõ (F05.2). "Coi như đến hạn" → 🟠 kèm nhãn ước lượng, không bao giờ 🔴 và không có email "Bạn đã tiêm chưa?"; "hỏi bác sĩ" → ⚪. Không có nguồn thì mặc định ⚪.
- "Đã có miễn dịch" không chọn được cho bệnh có nhắc lại định kỳ (F04.6).

### F06 — Bảng tiêm chủng theo độ tuổi

- **Bản công khai** (khách xem được, tốt cho SEO): hàng = bệnh (kèm hàng con theo dòng sản phẩm, ví dụ viêm não Nhật Bản 3 loại, HPV Gardasil / Gardasil 9, viêm gan A / A+B); cột = mốc tuổi (0–18 tháng theo tháng; 2–18 tuổi theo năm; nhóm 19–26, 27–45, 46–64, ≥ 65); ô có 3 trạng thái: **Lịch tiêm chuẩn / Lịch tiêm khuyến cáo (tiêm bù) / Không khuyến cáo**, kèm số mũi.
- **Bản cá nhân hoá**: tô cột tuổi hiện tại của hồ sơ và đánh dấu các mũi đã tiêm.
- **Mobile**: không hiển thị lưới; hiển thị danh sách theo nhóm tuổi, chạm để xem chi tiết. Lưới đầy đủ chỉ trên màn hình lớn.
- Bảng được sinh ra từ **cùng bộ quy tắc** mà bộ máy gợi ý dùng (một nguồn sự thật). Ô "Không khuyến cáo" của bảng chỉ khớp với khối "Không còn trong độ tuổi tiêm" đối với các trạng thái **dựa trên tuổi**; trạng thái phụ thuộc vào ngày tiêm của hồ sơ (mũi kế tiếp không còn kịp trước tuổi tối đa, F05.4b) không có ô tương ứng (F05.4a).
- Đầu bảng (cả bản công khai lẫn cá nhân hoá) có dòng miễn trừ chuẩn (F10.1 tầng 2). Bản công khai không cần tick miễn trừ (F02.2).

### F07 — Danh mục bệnh & vaccine

- **Bệnh**: tên, mô tả ngắn, các sản phẩm phòng bệnh, nguồn. Hiện "Có sản phẩm TCMR" nếu ít nhất một sản phẩm có nhãn TCMR còn hiệu lực.
- **Sản phẩm**: tên, nhà sản xuất, nước sản xuất, các bệnh phòng được (quan hệ nhiều-nhiều), phác đồ theo tuổi bắt đầu, thuộc tính vaccine sống, nguồn (tờ hướng dẫn sử dụng nếu có).
- **Nhãn TCMR** gắn theo **sản phẩm × nhóm đối tượng** (tuổi từ–đến, giới tính, năm sinh nếu có), có ngày hiệu lực từ–đến và ghi chú phạm vi — vì cùng một bệnh có thể có cả sản phẩm TCMR lẫn dịch vụ.
- Mỗi trang bệnh / sản phẩm có dòng miễn trừ chuẩn (F10.1 tầng 2).
- **Link "Xem tại VNVC / Long Châu"**: admin nhập tay URL trang sản phẩm. Không thu thập dữ liệu từ trang của họ.
- **Không có giá** trong MVP.

### F08 — Thông báo

- **Trung tâm thông báo trong app** (biểu tượng chuông) + **banner "sắp đến hạn"** trên trang gợi ý.
- **Email nhắc lịch**: gửi trước **7 ngày** và **1 ngày** so với ngày đến hạn (định nghĩa ở F05.4; với mũi bị giới hạn bởi tuổi tối đa là ngày đến hạn đã giới hạn, F05.4b). Thêm **một lần duy nhất** "Bạn đã tiêm chưa?" vào **ngày thứ 7 sau ngày đến hạn** nếu vẫn chưa có mũi; không lặp lại. Với khoảng đệm từ 7 ngày trở xuống, đây chính là ngày quá hạn (ngày đến hạn + max(khoảng đệm, 7 ngày), F05.4). Mục ở khối ngoài 5 nhóm (F05.4a, kể cả "Chưa tới lịch tiêm") và mục ⚪ không gửi email; mũi nhắc lại hiện 🟠 theo "coi như đến hạn" sau một mũi cuối không rõ không có email "Bạn đã tiêm chưa?" (F05.12).
  - **Gộp theo tài khoản**: mỗi ngày mỗi tài khoản nhận **tối đa một** email, gồm mọi mũi của mọi hồ sơ có ngày đến hạn đúng 7 ngày hoặc 1 ngày sau ngày gửi, hoặc **đã qua ngày đến hạn đúng 7 ngày** (ví dụ "Sắp đến hạn: Bé Na – Rota mũi 2; Mẹ – Cúm mùa"). Mỗi (mũi, ngày đến hạn, mốc) chỉ gửi một lần.
  - Nội dung được **tính lại lúc gửi** theo dữ liệu và phiên bản quy tắc hiện hành: mũi đã được ghi, hồ sơ đã xoá, hồ sơ đã tắt nhắc, hoặc hạn đã đổi thì không gửi theo hạn cũ.
  - Hạn dựa trên ngày ước lượng ghi kèm "(ước lượng)".
  - Chỉ gửi khi user đã đồng ý nhận email nhắc lịch (F02.3).
  - **Tiêu đề và dòng xem trước luôn trung tính** (ví dụ "VaxBook: bạn có lịch nhắc mới"): không có biệt danh, tên bệnh, tên vaccine.
  - Thân email không có ngày sinh, số lô, ghi chú, nơi tiêm; **không bao giờ** nhắc tới mang thai hay dự định mang thai; cuối email có dòng miễn trừ chuẩn.
  - Nội dung chỉ là nhắc lịch: **không** quảng cáo, không khuyến mãi.
- **Cài đặt thông báo**: bật / tắt email (chính là ghi hoặc rút đồng ý "Nhận email nhắc lịch"; bật lại phải tick đồng ý lại); bật / tắt nhắc lịch **cho từng hồ sơ**; mục "Thông báo đẩy" hiển thị nhãn **"Sắp có"**.
- Khách: chỉ có banner trong app, không email, không chuông.

**F08.1 Sự kiện thông báo**
| Sự kiện | Trong app | Email | Khách |
|---|---|---|---|
| Mũi đến hạn sau 7 ngày / 1 ngày | Có | Có (gộp) | Chỉ banner |
| Đã qua ngày đến hạn 7 ngày mà chưa ghi mũi ("Bạn đã tiêm chưa?", một lần) | Có | Có (gộp) | Chỉ banner |
| Xác nhận lại "Dự định mang thai" | Có | Không | Chỉ banner |
| Hỏi lại "Tôi chọn không tiêm lúc này" sau 12 tháng (F04.6) | Có | Không | Chỉ banner |
| "Hoãn đến [ngày]" đã tới ngày (F04.6) | Có | Không | Chỉ banner |
| Quy tắc / bệnh bị "Tạm ngưng tính lịch" (F11.8) có ảnh hưởng tới hồ sơ | Có (gộp với dòng dưới) | Không | Banner |
| Gợi ý của hồ sơ đổi nhóm hoặc đổi hạn do phiên bản quy tắc mới / rollback | Một thông báo gộp, kèm link ghi chú thay đổi (F10.2) | Không | Banner ở lần mở kế tiếp |

### F09 — Bản in lịch sử tiêm

- Trang in (in bằng trình duyệt / lưu PDF) toàn bộ lịch sử của **một hồ sơ**: bệnh, sản phẩm, ngày (kèm độ chính xác), số mũi, nơi tiêm, số lô.
- Chân trang: *"Do người dùng tự khai, không phải giấy xác nhận y tế"* + miễn trừ trách nhiệm.

### F10 — Minh bạch dữ liệu & miễn trừ trách nhiệm

**F10.1 Miễn trừ trách nhiệm 4 tầng**
1. Onboarding: tick bắt buộc, có phiên bản (F02.2).
2. **Dòng miễn trừ chuẩn** "Tham khảo · Nguồn: … · Hỏi bác sĩ trước khi tiêm" — một mẫu duy nhất, xuất hiện ở **mọi nơi hiển thị lịch hoặc gợi ý**: thẻ gợi ý, banner "sắp đến hạn", mục trong trung tâm thông báo, email nhắc lịch, đầu bảng theo độ tuổi (cả hai bản), trang bệnh / sản phẩm.
3. Chân trang bản in (F09).
4. Trang "Về dữ liệu" (F10.2).

**F10.2 Trang "Về dữ liệu"**
- Nguồn dữ liệu: Sổ tiêm chủng VNVC (bản in, năm phát hành chưa rõ); TT 10/2024/TT-BYT; TT 13/2026/TT-BYT; tờ hướng dẫn sử dụng sản phẩm.
- **Thứ tự ưu tiên nguồn**: văn bản Bộ Y tế > tờ hướng dẫn sử dụng > sổ / trang của chuỗi tiêm chủng. Khi có khác biệt, ghi chú rõ.
- Ngày cập nhật và số phiên bản bộ quy tắc hiện hành, kèm **ghi chú thay đổi** bằng ngôn ngữ thường của từng phiên bản (F11.5).
- Nói rõ: dữ liệu được rà soát bằng AI và người vận hành, **chưa được bác sĩ kiểm định** (cập nhật khi có).
- Giai đoạn beta: dữ liệu **không được sao lưu**, có thể mất khi máy chủ gặp sự cố; nên tải dữ liệu định kỳ. Khi có sao lưu: thời hạn dữ liệu đã xoá còn trong bản sao lưu (F01.4).
- Người vận hành **không khôi phục tài khoản thủ công** khi user mất quyền truy cập email (F01.6).
- Sản phẩm phi lợi nhuận, do cá nhân vận hành; cách liên hệ / báo lỗi.

**F10.3 "Báo sai dữ liệu"**
- Nút trên mỗi gợi ý, mỗi bệnh, mỗi sản phẩm, mỗi ô của bảng theo độ tuổi. Khách cũng gửi được.
- Báo cáo **chỉ lưu**: loại lỗi (sai liều / sai khoảng cách / sai tuổi / sai nhãn TCMR / sai nguồn / có thể gây hại / khác), mô tả, nguồn tham khảo (tuỳ chọn), mã đối tượng bị báo (quy tắc / bệnh / sản phẩm / ô bảng), số phiên bản bộ quy tắc lúc báo.
- **Không** lưu tài khoản, hồ sơ hay biệt danh. Ngay trên ô mô tả: "Vui lòng không ghi tên, ngày sinh hay thông tin sức khoẻ của người thật."
- Báo cáo vào hàng chờ của admin (F11.6); có thể chuyển thành changeset qua MCP (F12) sau khi được kiểm duyệt.

### F11 — Admin

- **F11.1** Chỉ admin (chủ dự án) truy cập. Trang admin chỉ quản lý danh mục, quy tắc và hàng chờ báo lỗi; **không có** màn hình xem, tìm hay sửa tài khoản, hồ sơ, lịch sử tiêm của user (chỉ có số đếm tổng hợp).
- **F11.2** Quản lý: bệnh, sản phẩm, quan hệ sản phẩm–bệnh, phác đồ, quy tắc (mọi loại ở F05.2), nhãn TCMR (theo sản phẩm × nhóm đối tượng, kèm ngày hiệu lực và ghi chú phạm vi), nguồn trích dẫn, link nơi cung cấp.
- **F11.3 Changeset**: mọi thay đổi dữ liệu y khoa / danh mục đi qua **changeset** (gói thêm / sửa / xoá). Xoá là **xoá mềm**.
  - Xoá mềm hoặc rollback **không xoá** thực thể đang được mũi tiêm của user tham chiếu: mũi tiêm vẫn hiện tên sản phẩm kèm "Không còn trong danh mục", được tính như không rõ sản phẩm (F05.3), và sản phẩm đó không còn để chọn cho mũi mới.
- **F11.3a Vòng đời changeset**: Nháp → Chờ duyệt → Đã duyệt → Đã xuất bản; cùng hai trạng thái Từ chối và Lỗi thời.
  - MCP chỉ sửa changeset ở trạng thái **Nháp**. Gửi duyệt thì khoá thành một **bản sửa có số hiệu**; mọi sửa đổi sau đó tạo bản sửa mới, đưa về Nháp và làm kết quả rà soát + kiểm thử cũ hết hiệu lực.
  - Admin duyệt đúng một số bản sửa.
  - Mỗi changeset ghi **phiên bản gốc**. Khi phiên bản hiện hành đã đổi → changeset chuyển **Lỗi thời**, phải cập nhật lên phiên bản mới (cảnh báo nếu cùng sửa một trường) và rà soát + kiểm thử lại.
- **F11.4 Duyệt**: xem diff so với phiên bản hiện hành, xem kết quả AI rà soát (F13) và kết quả chạy ca kiểm thử; duyệt hoặc từ chối. Trích dẫn tới nguồn "(CHƯA KIỂM CHỨNG)" hoặc đã bị thay thế được cảnh báo trên trang duyệt.
- **F11.5 Xuất bản & rollback**
  - Điều kiện xuất bản (đủ cả 4): (1) bản sửa cuối có kết quả rà soát từ **đủ 5 vai trò** (F13); (2) không còn phát hiện mức cao ở trạng thái Mở; (3) không còn ca kiểm thử thất bại; (4) mọi quy tắc tính lịch có ít nhất một trích dẫn ghi **vị trí cụ thể** (điều / khoản / trang).
  - Xuất bản tạo **phiên bản bộ quy tắc mới**, kèm **ghi chú thay đổi** bằng ngôn ngữ thường (hiện ở F10.2).
  - **Rollback** luôn tạo một phiên bản mới có nội dung bằng phiên bản đích, bắt buộc ghi lý do, không cần rà soát lại (phiên bản đích đã từng qua cổng); mọi changeset đang mở chuyển Lỗi thời. Rollback **giữ nguyên** mọi tạm ngưng tính lịch (F11.8) đang có hiệu lực, kể cả khi phiên bản đích tạo trước lần tạm ngưng; chỉ gỡ tạm ngưng qua changeset theo F11.8.
- **F11.6 Hàng chờ báo lỗi**: danh sách báo cáo từ F10.3; trạng thái Mới → **Đã kiểm duyệt** (admin đã đọc và che thông tin cá nhân nếu có) → Đang xử lý → Đã xử lý / Bỏ qua (bắt buộc lý do); liên kết tới changeset. Báo cáo loại "có thể gây hại" đứng đầu hàng chờ.
- **F11.7 Nhật ký** (chỉ ghi thêm, không sửa / xoá): mọi thay đổi kèm ai đề xuất (admin hay AI, vai trò nào), thời điểm, nguồn trích dẫn; duyệt / từ chối (kèm số bản sửa); xuất bản; rollback (kèm lý do); tạm ngưng / khôi phục tính lịch (kèm lý do); bác bỏ phát hiện (kèm lý do); mọi lệnh ghi của MCP.
- **F11.8 Tạm ngưng tính lịch (van an toàn khẩn cấp)**
  - Admin tạm ngưng tính lịch cho **một phác đồ (Series), một quy tắc đặc biệt (Rule) hoặc một bệnh** khi phát hiện lỗi nguy hiểm trong phiên bản đang xuất bản. Không tạm ngưng từng quy tắc mũi riêng lẻ (tránh phác đồ "đúng một nửa").
  - Tạm ngưng một quy tắc → chỉ các mục do quy tắc đó sinh ra; tạm ngưng một bệnh → mọi mục của bệnh đó. Các mục này hiện ở ⚪ với nhãn **"Tạm ngưng tính lịch — tham khảo bác sĩ"**, trừ khi một lớp cao hơn ở F05.11 áp dụng (lớp 1 bệnh chỉ thông tin → "Chỉ lưu trữ"; lớp 2 ghi nhận còn hiệu lực → "Đã ghi nhận"). Ô tương ứng trong bảng F06 ghi "Tạm ngưng — hỏi bác sĩ". **Huỷ ngay** email nhắc liên quan.
  - Thao tác tạo một phiên bản mới, **bắt buộc ghi lý do**, và **được miễn** điều kiện xuất bản F11.5 (miễn AI rà soát và kiểm thử) vì chỉ làm gợi ý thận trọng hơn (P1).
  - Khôi phục tính lịch phải đi qua changeset bình thường (đủ điều kiện F11.5).
  - Chỉ admin làm được, trong trang admin; MCP không có quyền này.

### F12 — MCP server cho admin

- **Chỉ admin**, xác thực bằng khóa riêng.
- Phạm vi: **chỉ danh mục và bộ quy tắc**. **Không** truy cập dữ liệu người dùng.
- Công cụ:
  - Đọc danh mục, quy tắc, các phiên bản, changeset, và báo cáo lỗi ở trạng thái **Đã kiểm duyệt** (F11.6).
  - **Tạo / cập nhật changeset** (chỉ ở trạng thái Nháp) chứa thao tác thêm / sửa / xoá.
  - **Thêm** kết quả rà soát vào changeset (không sửa / xoá kết quả đã có).
  - Chạy bộ máy gợi ý trên **hồ sơ giả lập** (ca kiểm thử).
  - So sánh quy tắc với văn bản nguồn.
- Thay đổi **kết quả kỳ vọng** của ca kiểm thử là một thao tác riêng trong changeset, được gắn nhãn "Đổi kỳ vọng kiểm thử" trong diff.
- **Không được**: xuất bản, duyệt, rollback, tạm ngưng tính lịch, đổi trạng thái phát hiện. Các thao tác này chỉ admin làm trong trang admin.

### F13 — AI rà soát đa vai trò

| Vai trò | Kiểm tra |
|---|---|
| Bác sĩ nhi / y học dự phòng | Logic lâm sàng, quy tắc tiêm bù |
| Dược sĩ | Đối chiếu tờ hướng dẫn sử dụng: liều, tuổi tối thiểu / tối đa, khoảng cách mũi |
| Pháp chế y tế | Đối chiếu TT 10/2024, TT 13/2026; nhãn TCMR |
| Kiểm thử dữ liệu (QA) | Tính nhất quán; sinh ca kiểm thử (ví dụ "bé 7 tháng chưa tiêm Rota → quá tuổi bắt đầu") |
| Biên tập nội dung | Văn phong trung lập, không như chỉ định y khoa, dễ hiểu |

- **Thời điểm**: bắt buộc trước mỗi lần xuất bản; khi admin yêu cầu; định kỳ hằng tháng rà soát văn bản mới.
- Kết quả: danh sách phát hiện có **mức độ** (cao / trung bình / thấp) và **nguồn trích dẫn**. Phát hiện không kèm nguồn được hiển thị là "chưa có căn cứ".
- Bất kỳ phát hiện mức **cao** nào ở trạng thái Mở → chặn xuất bản (F11.5).
- AI rà soát **không thay thế** người có chuyên môn (xem §8).

**F13.1 Vòng đời phát hiện**
- Trạng thái: **Mở** → **Đã sửa** (liên kết bản sửa changeset) hoặc **Bác bỏ** (bắt buộc lý do; với mức cao phải kèm nguồn phản bác).
- Chỉ admin đổi trạng thái phát hiện, trong trang admin.

**F13.2 Ca kiểm thử bắt buộc tối thiểu** (QA sinh và duy trì; mọi changeset phải chạy qua)
- Viêm gan B sơ sinh + 6in1 × 3 → ✅, không cảnh báo đổi loại.
- Rota không rõ sản phẩm → phác đồ an toàn nhất (F05.3).
- Rotarix mũi 1 + RotaTeq mũi 2 → chưa hoàn thành, có cảnh báo đổi loại.
- Bé 7 tháng chưa tiêm Rota → không còn trong độ tuổi bắt đầu.
- Mũi tiêm trước tuổi tối thiểu / chưa đủ khoảng cách → không được tính (F05.10).
- Ngày tiêm ước lượng chồng lấn → ⚪ (F04.2).
- Gardasil 9 bắt đầu ở tuổi ước lượng 14–15 → phác đồ 3 mũi, nhãn ước lượng (F05.5).
- Đang mang thai + vaccine sống đến hạn → khối "Hoãn trong thai kỳ", không email (F05.7).
- Hồ sơ "Chưa nhập xong", bé 6 tháng, 0 bản ghi → mọi mốc đến hạn trước thời điểm chọn lựa chọn này là ⚪ "Chưa nhập", không có 🔴, trừ các bệnh chắc chắn đã quá tuổi, vào khối "Không còn trong độ tuổi tiêm" (F03.6, F05.11).
- Hồ sơ "Chưa nhập xong", mốc đến hạn **sau** thời điểm chọn lựa chọn này → tính bình thường (🟠 / 🔴, có email) (F03.6).
- Bé 5 tuần, 6in1 sớm nhất lúc 6 tuần, đến hạn lúc 2 tháng → 🟡 (F05.4).
- Rollback về phiên bản trước lần tạm ngưng → tạm ngưng vẫn giữ (F11.5).
- Người lớn 45 tuổi, lựa chọn 1 hoặc 2, chưa có bản ghi → nút nhanh "Bạn đã từng tiêm chưa?" cho các bệnh còn trong độ tuổi tiêm (ví dụ viêm gan B), không có 🔴 cho tới khi chọn "Chưa tiêm"; các bệnh chắc chắn đã quá tuổi (ví dụ Rota, Hib) vào khối "Không còn trong độ tuổi tiêm" (F03.6, F05.11).
- Người lớn 45 tuổi, lựa chọn 3 "Chưa tiêm mũi nào" → tính bình thường, có thể có 🔴 (F03.6).
- Đã tiêm ≥ 1 mũi, mũi kế tiếp còn hơn 6 tháng → ✅ "Đang theo phác đồ" (F05.4).
- "Đã có miễn dịch" cho uốn ván → không cho chọn (F04.6).
- Khoảng đệm 0: hôm nay = ngày đến hạn → 🟠; hôm nay = ngày đến hạn + 7 ngày → 🔴 (F05.4).
- Chưa tiêm mũi nào, mốc đầu tiên còn hơn 6 tháng → khối "Chưa tới lịch tiêm", không email (F05.4a).
- Phác đồ có tuổi tối đa hoàn thành và một mốc tính từ mũi đầu rơi vào sau tuổi đó (ví dụ tuổi tối đa hoàn thành 20 tuổi, mũi 2 vào mũi đầu + 12 tháng, mũi 1 lúc 19 tuổi 7 tháng) → 🟠 "hạn chót", `DUE_CAPPED_BY_MAX_AGE`, không bao giờ 🔴; cùng ca đó với ngày sinh chỉ có tháng/năm thì dùng đầu già (F05.4b). Đây là dữ liệu đầu vào kiểm thử, không phải giá trị y khoa.
- RotaTeq mũi 1 lúc 14 tuần, hôm nay 29 tuần → mũi 2 vẫn hiện, `SERIES_MAY_NOT_COMPLETE` (F05.4b).
- Rotarix mũi 1 + RotaTeq mũi 2, hôm nay đã qua giới hạn của Rotarix nhưng chưa tới giới hạn của RotaTeq → ⚪ `MIXED_PRODUCTS` (F05.3).
- Bộ quy tắc có các phương án thay thế với số mũi khác nhau trong một phác đồ → bị kiểm tra bộ quy tắc từ chối (F05.2).
- Rotarix mũi 1 lúc 16 tuần 4 ngày → ⚪ `DOSE_AFTER_MAX_AGE` (F05.10).
- HPV mũi 1 có ngày, mũi 2 mức 4, mũi 3 có ngày và có cách đặt khả thi → ✅ ước lượng, `INTERVAL_UNVERIFIABLE`; không có cách đặt khả thi → ⚪ `DOSE_INVALID` (F05.10).
- Không có ngày sinh, đã tiêm sởi mũi 1 → mũi 2 đến hạn = mũi 1 + 9 tháng, ước lượng (F05.5).
- Ngày sinh chỉ có năm, nằm vắt qua ranh giới HPV 9–14 / 15–45 tuổi, chưa tiêm mũi nào → mỗi đầu dùng nhóm tuổi bắt đầu của riêng nó (F05.5).
- Trẻ 3 tuổi có một mũi Rota ở mức 4 → khối "Không còn trong độ tuổi tiêm" (F05.11).
- "Chưa nhập xong", đã nhập 1 trong 3 mũi ho gà → vẫn "Chưa nhập xong", mũi 2 vẫn là ⚪ "Chưa nhập" (F03.6).
- Chọn "Không nhớ thời nhỏ" cho trẻ dưới 1 tuổi; một mốc thơ ấu đến hạn sau thời điểm đó → tính bình thường (F04.5).
- Sản phẩm ngoài danh mục, phác đồ đủ theo danh mục → ✅ kèm `PRODUCT_NOT_IN_CATALOG` (F05.3).
- Ngày sinh chỉ có tháng/năm, một đầu 🟡 và đầu kia "Chưa tới lịch tiêm" → không phải ⚪ (F05.5).
- "Đã có miễn dịch" cho một bệnh đã có mũi tiêm → "Đã ghi nhận" (F04.6).
- Hết khung mùa của mũi nhắc lại theo mùa mà chưa tiêm → sang mùa sau, không bao giờ 🔴 (F05.12).
- Đang mang thai, cùng một bệnh có cả sản phẩm vaccine sống và không sống → chỉ liệt kê các sản phẩm không phải vaccine sống (F05.7).

---

## 5. Mô hình dữ liệu khái niệm

> Mức khái niệm để SAD chi tiết hoá; không phải schema.

```
── Dữ liệu người dùng ───────────────────────────────────────────────────────────
Account ──< ProfileMembership >── Profile
              (role, is_self,        ├──< DoseRecord ──< DoseRecordDisease >── Disease
               notify_enabled)       │     (product_id tuỳ chọn, tên sản phẩm tự do,      (số thứ tự mũi
                                     │      ngày + độ chính xác, nơi tiêm, số lô,           theo bệnh)
                                     │      ghi chú, nguồn bản ghi = "tự nhập")
                                     ├──< DiseaseDeclaration (bệnh, ngày tạo,
                                     │      loại = về lịch sử: chưa rõ / đã xác nhận chưa tiêm
                                     │      | ghi nhận: đã có miễn dịch / bác sĩ khuyên không tiêm /
                                     │        hoãn đến ngày / chọn không tiêm + ngày hỏi lại;
                                     │      lý do = mức 5 / không nhớ thời nhỏ / nút nhanh / ghi nhận của user)
                                     ├── PregnancyStatus (đang mang thai + ngày dự sinh / dự định + ngày xác nhận lại)
                                     └── (biệt danh, ngày sinh + độ chính xác, giới tính,
                                          tình trạng lịch sử = chưa nhập xong / không nhớ thời nhỏ / chưa tiêm / đã nhập xong,
                                          kèm thời điểm chọn)

── Danh mục & quy tắc (toàn bộ thuộc một RuleSetVersion) ─────────────────────────
RuleSetVersion ──< Disease, Product (thuộc tính vaccine sống), ProductDisease,
                   Series (bắt buộc gắn bệnh; sản phẩm tuỳ chọn; khoảng tuổi bắt đầu — bắt buộc khi một cặp (bệnh, sản phẩm)
                           có nhiều phác đồ; giai đoạn; tuổi tiêm bù tuỳ chọn; phương án thay thế, cùng số mũi)
                     ──< Dose rule (mốc khuyến cáo, tuổi tối thiểu / tối đa, khoảng cách tối thiểu từ mũi trước /
                                    mũi đầu, khoảng đệm, dung sai),
                   Rule (cấp bệnh: nhắc lại định kỳ — mỗi N tháng hoặc theo khung mùa, hành vi khi mũi cuối
                         không rõ; trước mang thai; mũi bổ sung được phép; yêu cầu cùng sản phẩm;
                         chỉ thông tin),
                   TCMR label (sản phẩm × nhóm đối tượng, hiệu lực từ–đến, ghi chú phạm vi),
                   Citation ──> SourceDocument (vị trí cụ thể: điều / khoản / trang),
                   ProviderOffering ──> Provider (VNVC, Long Châu)   [MVP: chỉ URL]
                   + ghi chú thay đổi
Changeset (phiên bản gốc, trạng thái) ──< Revision ──< ChangeOperation (thêm / sửa / xoá mềm)
                                                   ──< AIReviewFinding (vai trò, mức độ, nguồn, trạng thái)
                                                   ──< TestRun ──> TestCase (hồ sơ giả lập, kết quả kỳ vọng)

── Khác ──────────────────────────────────────────────────────────────────────────
FeedbackReport (không gắn account / profile), Notification, EmailDispatch (mũi × ngày đến hạn × mốc đã gửi),
ConsentRecord (loại, phiên bản văn bản, thời điểm, cấp tài khoản hoặc cặp tài khoản–hồ sơ, đồng ý / rút),
DisclaimerAcceptance (phiên bản), RuleSuspension (phác đồ / quy tắc / bệnh, lý do — mang sang mọi phiên bản sau, kể cả phiên bản tạo bởi rollback, cho tới khi khôi phục qua changeset), AuditLog (chỉ ghi thêm)
```

Các điểm then chốt:
- `Profile` là **thực thể độc lập**, không thuộc trực tiếp về `Account`. Quan hệ qua `ProfileMembership`.
- `is_self` ("Tôi") và cài đặt nhắc lịch nằm ở **membership**, không nằm ở `Profile` — vì "Tôi" là quan hệ giữa người dùng và hồ sơ.
- Mọi dữ liệu y tế (`DoseRecord`, `PregnancyStatus`, cờ…) gắn với `profile_id`, **không bao giờ** gắn với `account_id`.
- Mũi tiêm gắn với **bệnh** qua `DoseRecordDisease` (bắt buộc ≥ 1); sản phẩm là tuỳ chọn.
- `DiseaseDeclaration` có hai loại với thứ tự ưu tiên khác nhau (F04.6): khai báo **về lịch sử** nhường cho mũi tiêm của bệnh và được ưu tiên hơn tình trạng lịch sử; khai báo **ghi nhận** thắng khi còn hiệu lực, kể cả khi đã có mũi tiêm. Ngày tạo giới hạn các mốc mà khai báo "chưa rõ" áp dụng (F04.2).
- Gợi ý luôn tham chiếu `RuleSetVersion` đã dùng để tính.
- Mỗi thực thể danh mục có **ID ổn định, không tái sử dụng** qua các phiên bản, để mũi tiêm của user luôn tham chiếu được.
- Ngày sinh và ngày tiêm luôn lưu **kèm độ chính xác**.
- Dữ liệu người dùng xoá cứng; danh mục & quy tắc xoá mềm.

---

## 6. Thiết kế cho mở rộng

| Tính năng tương lai | Chuẩn bị trong MVP |
|---|---|
| **Giá & đồng bộ giá** | `Provider` và `ProviderOffering` đã có; sau này thêm `PriceSnapshot` (giá, loại giá — lẻ / đặt trước / gói, ngày ghi nhận, nguồn) gắn vào `ProviderOffering`. Nguồn dữ liệu giá phải là một lớp thay được (nhập tay → nguồn tự động khi được phép) |
| **Web push** | Mô hình thông báo tách "nội dung" khỏi "kênh gửi"; cài đặt đã có mục "Thông báo đẩy — Sắp có" |
| **Chia sẻ hồ sơ** | `ProfileMembership` nhiều-nhiều với `role` (MVP chỉ `owner`; sau thêm `editor`, `viewer`) |
| **Chuyển hồ sơ sang tài khoản khác** | Chỉ là thay đổi `ProfileMembership` (thêm owner mới, gỡ owner cũ). Dữ liệu y tế không phải di chuyển vì gắn với `profile_id` |
| **Tách hồ sơ thành tài khoản riêng** | Tạo `Account` mới (qua lời mời email) + chuyển membership như trên; `is_self` đặt trên membership của tài khoản mới |
| **Đồng thuận khi chuyển / tách** | `ConsentRecord` theo cặp (account, profile) **đã có từ MVP** (F03.5); tài khoản mới chỉ cần ghi bản đồng ý của mình |
| **Nhật ký sở hữu** | `AuditLog` ghi mọi thay đổi membership |
| **Đăng nhập Google** | Tài khoản định danh bằng email; phương thức đăng nhập là thành phần tách rời |
| **OCR ảnh sổ, đồng bộ VNeID** | `DoseRecord` có trường "nguồn bản ghi" (tự nhập / OCR / đồng bộ) |
| **Bệnh nền, du lịch, nghề nghiệp** | Bộ máy quy tắc nhận tập "điều kiện" mở rộng được (MVP chỉ có mang thai / dự định mang thai) |
| **Tiếng Anh** | Nội dung danh mục / quy tắc tách khỏi giao diện để dịch được |
| **Sao lưu bộ quy tắc qua git** (ưu tiên cao, sau MVP) | Endpoint xuất mọi phiên bản / changeset / phát hiện (không có dữ liệu cá nhân); GitHub Action hằng ngày commit vào `rules-data/`; lệnh khôi phục (SAD §14) |

---

## 7. Dữ liệu khởi tạo

1. **Sổ tiêm chủng VNVC (bản in)** — AI chép thành dữ liệu có cấu trúc → các vai trò AI kiểm tra chéo → admin duyệt.
2. **TT 10/2024/TT-BYT** và **TT 13/2026/TT-BYT** — đối chiếu nhãn TCMR; bổ sung phế cầu, HPV. Lịch tiêm chi tiết PCV / HPV trong phụ lục TT 13/2026 **(CHƯA KIỂM CHỨNG)**.
3. **Tờ hướng dẫn sử dụng** — cho Qdenga, Shingrix, RSV (không có trong sổ) và phác đồ từng sản phẩm.
4. **Dại, tả** — chỉ hiển thị thông tin, không tính lịch.
5. Khi sổ VNVC khác văn bản Bộ Y tế → **văn bản Bộ Y tế được ưu tiên** và ghi chú khác biệt.

---

## 8. Rủi ro & cách giảm thiểu

| Rủi ro | Mức | Giảm thiểu |
|---|---|---|
| **Sai quy tắc y khoa** (liều, khoảng cách, tuổi) | Cao nhất | Nguyên tắc P1; trích nguồn; AI rà soát 5 vai trò + ca kiểm thử; chặn xuất bản khi có lỗi cao; nút báo sai; miễn trừ trách nhiệm 4 tầng; khuyến nghị một lần bác sĩ / dược sĩ kiểm tra trước khi public |
| **Bảo vệ dữ liệu cá nhân** | Trung bình | Luật BVDLCN 91/2025 và Nghị định 356/2025 áp dụng theo việc xử lý dữ liệu, không theo mục đích lợi nhuận; chưa thấy miễn trừ cho cá nhân **(CHƯA KIỂM CHỨNG)**. Giảm thiểu: thu thập tối thiểu (P4), đồng ý tách mục, xoá được, mã hoá, không đưa dữ liệu user cho AI/MCP |
| **Dữ liệu lỗi thời** (thông tư mới, sản phẩm mới) | Trung bình | AI rà soát văn bản mới hằng tháng; hiển thị ngày cập nhật bộ quy tắc |
| **User bỏ đi** (mũi tiêm cách nhau hàng tháng / năm) | Trung bình | Email nhắc gộp theo tài khoản; không dội cảnh báo đỏ (P5) |
| **AI rà soát sai một cách tự tin** | Trung bình | AI không có quyền xuất bản; admin duyệt mọi changeset; phát hiện phải kèm nguồn |

---

## 9. Ra mắt

- **Beta kín** theo lời mời (vài chục gia đình quen biết): AI rà soát + admin duyệt là đủ.
- **Public**: khuyến nghị thêm một lần bác sĩ hoặc dược sĩ kiểm tra bộ quy tắc. Chủ dự án quyết định thời điểm.

---

## 10. Việc chuyển sang SAD

Các chủ đề SAD cần quyết định (không thuộc tài liệu này):
- Kiến trúc tổng thể, công nghệ front-end / back-end, hosting (trong nước hay nước ngoài).
- Lưu trữ dữ liệu khách trên trình duyệt và cơ chế chuyển lên tài khoản.
- Thiết kế bộ máy quy tắc (biểu diễn quy tắc, đánh giá, kiểm thử, phiên bản).
- Dịch vụ gửi email, lịch gửi nhắc.
- Kiến trúc MCP server và luồng AI rà soát.
- Bảo mật, mã hoá, xác thực admin / MCP.

---

## Phụ lục A — Nhật ký quyết định

| # | Quyết định |
|---|---|
| Q1, Q27, Q43, Q44 | Một tài khoản nhiều hồ sơ; hỗ trợ người dùng cá nhân; giao diện tự suy ra từ số hồ sơ; hồ sơ "Tôi" tuỳ chọn |
| Q2 | Công cụ tham khảo, có miễn trừ trách nhiệm, trích nguồn |
| Q3 | Không đồng bộ lịch sử từ VNeID / tài khoản chuỗi; user tự nhập |
| Q4, Q18, Q19 | Giá bỏ khỏi MVP; thiết kế sẵn để thêm giá VNVC + Long Châu sau |
| Q5 | Phi lợi nhuận |
| Q6 | Gợi ý theo tuổi, giới, lịch sử, khoảng cách, tiêm bù + cờ mang thai |
| Q7, Q22 | Chế độ khách; chỉ đăng nhập email OTP |
| Q8, Q29, Q45 | Không web push; trung tâm thông báo + email nhắc 7 ngày / 1 ngày, gộp theo tài khoản, bật / tắt theo hồ sơ |
| Q9 | Web mobile-first, tiếng Việt, PWA cơ bản |
| Q10 | Độ chính xác ngày tiêm 5 mức, tính theo hướng an toàn |
| Q11 | Chọn bệnh trước, sản phẩm tuỳ chọn, hỗ trợ mũi phối hợp |
| Q12 | Gợi ý lai theo bệnh / sản phẩm, cảnh báo đổi hãng |
| Q13 | 5 nhóm kết quả, nhãn TCMR / Dịch vụ (không hiển thị giá) |
| Q14 | Dữ liệu y khoa do admin duyệt, có phiên bản và nguồn |
| Q15 | Chưa chia sẻ hồ sơ; data model nhiều-nhiều |
| Q16 | Bản in lịch sử tiêm |
| Q17, Q36 | Tên tạm: VaxBook (đã có app trùng tên ở Nam Phi; vaxbook.com đã có người đăng ký) |
| Q20 | Không hỏi tỉnh; ghi chú TCMR có thể chưa triển khai tại địa phương |
| Q21 | TCMR (11 bệnh TT 10/2024 + phế cầu + HPV) + vaccine dịch vụ phổ biến được tính lịch; RSV chỉ thông tin |
| Q23 | Cờ mang thai có ngày dự sinh, tự hết hạn |
| Q24 | Chuyển dữ liệu khách sang tài khoản; xoá từng hồ sơ |
| Q25 | Vận hành cá nhân; beta kín trước |
| Q26, Q33, Q34, Q39 | MCP cho admin: thêm / sửa / xoá qua changeset; AI rà soát 5 vai trò; admin duyệt và xuất bản |
| Q28 | "Không nhớ lịch sử thời nhỏ" → nhóm "Nên hỏi bác sĩ hoặc xét nghiệm" |
| Q30 | Danh mục không giá, có link VNVC / Long Châu |
| Q31, Q40 | Dữ liệu khởi tạo từ sổ VNVC; văn bản Bộ Y tế được ưu tiên khi khác biệt |
| Q32 | Bảng theo độ tuổi công khai + cá nhân hoá; danh sách trên mobile |
| Q35 | Public khuyến nghị thêm một lần chuyên gia kiểm tra |
| Q37 | Thu thập tối thiểu: email, biệt danh, ngày sinh (4 mức), giới tính (tuỳ chọn); không ảnh |
| Q38 | Miễn trừ trách nhiệm 4 tầng, trang "Về dữ liệu", nút "Báo sai dữ liệu" |
| Q42 | Xử lý ngày sinh / giới tính không đầy đủ |
| (sau Q45) | Thiết kế sẵn tách hồ sơ thành tài khoản riêng / chuyển hồ sơ sang tài khoản khác |
| Rà soát 1 | 13 chỉnh sửa trực tiếp (định nghĩa ngày đến hạn, phác đồ an toàn nhất, mũi không hợp lệ, vòng đời changeset, email…) |
| Q46 | Câu hỏi tình trạng lịch sử khi tạo hồ sơ; "Chưa nhập xong" hiện ⚪; người lớn có nút nhanh "Bạn đã từng tiêm chưa?" |
| Q47 | Khối ngoài 5 nhóm: "Không còn trong độ tuổi tiêm", "Chỉ lưu trữ", "Đã ghi nhận"; mũi > 6 tháng vào ✅ "Đang theo phác đồ" |
| Q48 | Email "Bạn đã tiêm chưa?" một lần vào ngày thứ 7 sau hạn |
| Q49 | Ghi nhận theo bệnh: miễn dịch / bác sĩ khuyên không tiêm / hoãn / chọn không tiêm |
| Q50 | Không đồng ý lưu → chế độ khách; rút đồng ý → xoá dữ liệu server, giữ tài khoản; mục "Quyền riêng tư" |
| Q51 | Van an toàn "Tạm ngưng tính lịch" cho admin |
| Q52 | Đổi email, tải dữ liệu của tôi, không khôi phục thủ công |
| Q53 | Xác nhận đồng ý theo hồ sơ cho người lớn khác; xác nhận chung cho trẻ em |
| Rà soát hồi quy | 7 chỉnh sửa (khoảng trống 🟠/🟡, rollback giữ tạm ngưng, mốc thời gian của "Chưa nhập xong", trạng thái "đã xác nhận chưa tiêm"…) |
| Q54 | Nút nhanh "Bạn đã từng tiêm chưa?" cho người lớn ở lựa chọn 1 và 2; lựa chọn 3 tính bình thường |

---

## Phụ lục B — Thay đổi v1.1 (2026-11-10, theo SAD)

| # | Thay đổi | Nguồn |
|---|---|---|
| B1 | Thuật ngữ: `ProfileDiseaseStatus` / "trạng thái bệnh" → `DiseaseDeclaration` / "khai báo theo bệnh"; thêm Assessment, Bucket, Block, Next dose | GLOSSARY.md, SAD Q7, Q15 |
| B2 | F01.4, F10.2: beta không có bản sao lưu; nói rõ trên trang "Về dữ liệu" | SAD Q22, Q31, ADR-0007 |
| B3 | F11.8: đối tượng tạm ngưng là Series, Rule hoặc Disease | SAD Q8 |
| B4 | §6: thêm "Sao lưu bộ quy tắc qua git" (ưu tiên cao, sau MVP) | SAD Q30 |
| — | 13 khoảng trống ngữ nghĩa bộ máy gợi ý do prototype phát hiện (SAD §5.6 B1–B13) **đã được chốt ở v1.2** (Phụ lục C) | SAD §5.6 |

---

## Phụ lục C — Thay đổi v1.2 (2026-11-11, ngữ nghĩa bộ máy gợi ý)

Các quyết định EQ1–EQ27 từ buổi chốt ngữ nghĩa bộ máy gợi ý (chủ dự án đã duyệt). Đánh số EQ để không trùng với Q1–Q54 (Phụ lục A) và B1–B4 (Phụ lục B). Khi một mục sau làm rõ một mục trước, mục sau được ưu tiên.

| # | Quyết định (tóm tắt) | Chốt SAD §5.6 | Mục đặc tả thay đổi |
|---|---|---|---|
| EQ1 | Ngày quá hạn = ngày đến hạn + max(khoảng đệm, 7 ngày); 7 ngày là hằng số hiển thị của bộ máy; 🟠 vào đúng ngày đến hạn, 🔴 từ ngày đến hạn + 7; mũi bị giới hạn không bao giờ 🔴 | B1 | F05.2, F05.4, F08 |
| EQ2 | Chưa tiêm mũi nào và mũi 1 đến hạn sau hơn 6 tháng → khối mới "Chưa tới lịch tiêm": thu gọn, không tô đỏ, không email, không tính mức độ cần chú ý | B2 | F03.4, F05.4, F05.4a, F08 |
| EQ3 | Ngày đến hạn sau tuổi tối đa → `DUE_CAPPED_BY_MAX_AGE` (được thay bằng EQ15) | B3a | F05.4b |
| EQ4 | Phác đồ không còn hoàn thành kịp → `SERIES_MAY_NOT_COMPLETE`, chỉ mang tính thông tin (được thay bằng EQ15) | B3b | F05.4b |
| EQ5 | Mọi phương án thay thế trong một phác đồ phải có cùng số mũi (kiểm tra bộ quy tắc); nếu không: tách phác đồ theo sản phẩm / khoảng tuổi bắt đầu không chồng lấn, chọn một phương án, hoặc không tính lịch | B4 | F05.2, F05.4a |
| EQ6 | Thứ tự gộp: phương án thay thế trong từng phác đồ → ngày muộn nhất giữa các phác đồ ứng viên → giới hạn tuổi tối đa | B5 | F05.3, F05.4a |
| EQ7 | Mũi tiêm sau tuổi tối đa → ⚪ `DOSE_AFTER_MAX_AGE`, không được tính; vi phạm ở một đầu → `DOSE_POSSIBLY_INVALID`; kiểm tra theo từng ứng viên; khoảng ngày mức 2–3 giới hạn tới hôm nay cho xếp thứ tự, kiểm tra và tính mũi kế tiếp | B7a | F04.2, F05.10 |
| EQ8 | Mũi sau một mũi mức 4 / mũi ngầm định: bỏ qua khoảng cách tới mũi trước không có ngày, mũi được tính, ước lượng, `INTERVAL_UNVERIFIABLE`; không bao giờ kiểm tra mũi j so với j−2 | B7b | F04.2, F05.10 |
| EQ9 | Không có ngày sinh (được thay bằng EQ19) | B8 | F05.5 |
| EQ10 | Chưa tiêm mũi nào: mỗi đầu khoảng ngày sinh dùng nhóm tuổi bắt đầu của riêng nó; quy tắc "nhiều mũi hơn" chỉ khi đã có mũi tiêm | B9 | F05.3, F05.5 |
| EQ11 | Chắc chắn đã quá tuổi thắng ⚪ do mức 4 | B10 | F04.2, F05.11 |
| EQ12 | "Chưa nhập xong" thoát bằng nút hoặc khi không còn đánh giá nào là `NOT_ENTERED` (theo từng mốc); bộ máy trả về các bệnh còn chờ nhập | B11 | F03.6 |
| EQ13 | "Không nhớ lịch sử thời nhỏ" chỉ áp dụng cho các mốc thơ ấu đến hạn trước thời điểm chọn | B12 | F04.5 |
| EQ14 | Sản phẩm khả dĩ = danh mục, kể cả sản phẩm đã ngừng; mũi ngoài danh mục mà phác đồ đủ theo danh mục → ✅ ước lượng, `PRODUCT_NOT_IN_CATALOG` | B13 | F05.3, F05.4 |
| EQ15 | Giới hạn tuổi tối đa thống nhất: hạn chót L / L_k ở đầu già, L nhỏ nhất giữa các ứng viên, áp dụng sau bước gộp; mũi bị giới hạn 🟠 "hạn chót", không bao giờ 🔴; `SERIES_MAY_NOT_COMPLETE`; mở rộng khối quá tuổi; đồng bộ F06 chỉ cho trạng thái dựa trên tuổi | B3, B6 | F05.3, F05.4, F05.4a, F05.4b (mới), F06, F08 |
| EQ16 | Ứng viên không thống nhất về tuổi tối đa: `MIXED_PRODUCTS` khi mọi mũi đều có tên sản phẩm, `CHECK_PRODUCT_NAME` chỉ khi có sản phẩm không rõ | B6 | F05.3, F05.10 |
| EQ17 | So sánh hai đầu: khác biệt chỉ về khoảng cách (khối / 🟡 / ✅ đang theo phác đồ) không phải kết quả khác nhau; ngày theo tuổi tối thiểu lấy từ đầu muộn hơn, giới hạn tuổi tối đa lấy từ đầu già; đầu muộn hơn không vượt quá hôm nay | — (SAD §5.3 bước 4) | F05.5 |
| EQ18 | Tên khối và nhãn mục cho "Chưa tới lịch tiêm"; mục vaccine sống của hồ sơ đang mang thai hiện "sau sinh — hỏi bác sĩ" | — (làm rõ B2) | F05.4a, F05.7 |
| EQ19 | Không có ngày sinh: chỉ thay các mốc tính từ ngày sinh, giữ khoảng cách giữa các mốc; bỏ kiểm tra theo tuổi, giữ kiểm tra khoảng cách; các trường hợp `NO_BIRTH_DATE` | B8 | F05.5 |
| EQ20 | Mũi mức 4 / mũi ngầm định: có số thứ tự thì nằm đúng số đó, không có số thì không có vị trí; chỉ bỏ qua khoảng cách tới mũi trước; không có cách đặt khả thi → `DOSE_INVALID`; ngày thay thế cho mũi đầu `FIRST_DOSE_DATE_ASSUMED`; luôn ước lượng | — (làm rõ B7b) | F04.1, F04.2, F05.10 |
| EQ21 | Thứ tự ưu tiên tám tầng; chắc chắn đã quá tuổi thắng mặc định theo lịch sử, ⚪ do mức 4 và (có điều kiện) tính hợp lệ của mũi | B10 (bổ sung SAD §5.3 bước 10) | F03.6, F04.5, F05.4, F05.11 (mới) |
| EQ22 | Chi tiết tự động thoát: danh sách chờ nhập tính trước tạm ngưng / mang thai; chỉ xét sau khi user lưu; không có ngày sinh → chỉ bằng nút; thiếu thời điểm là đầu vào không hợp lệ | — (làm rõ B11) | F03.6, F05.1 |
| EQ23 | Hai loại khai báo theo bệnh: về lịch sử (nhường cho mũi tiêm) và ghi nhận (thắng khi còn hiệu lực); hết hạn hoãn thì quay lại kèm banner; "chọn không tiêm lúc này" chờ lần hỏi lại; mốc cắt của mức 5 theo ngày tạo | — (sửa SAD §5.6 bảng C) | F03.6, F04.2, F04.6, §5 |
| EQ24 | Nhắc lại định kỳ: bắt đầu sau phác đồ cơ bản, số thứ tự tiếp nối; công thức mỗi N tháng và theo mùa; theo mùa không bao giờ 🔴; bệnh chỉ có nhắc lại = phác đồ một mũi + nhắc lại; mũi cuối không rõ theo quy tắc | — | F05.2, F05.4, F05.12 (mới), F08, §5 |
| EQ25 | Mang thai: loại sản phẩm vaccine sống khỏi danh sách; quy tắc trước mang thai chỉ thêm ghi chú; ghi chú sau sinh kéo dài 3 tháng; `LIVE_VACCINE_BEFORE_PREGNANCY` | — | F05.3, F05.7 |
| EQ26 | Mũi hợp lệ theo quy tắc tại ngày tiêm hoặc hôm nay; mũi kế tiếp dùng quy tắc hôm nay; `NO_SERIES_IN_EFFECT`; bắt buộc tuổi bắt đầu khi một cặp (bệnh, sản phẩm) có nhiều phác đồ; `catchUpAfterAge` tuỳ chọn → `CATCH_UP_ASK_DOCTOR` | — | F05.1, F05.2, F05.10, §5 |
| EQ27 | Làm rõ câu chữ: hằng số hiển thị không cần trích nguồn; dung sai chỉ dùng khi kiểm tra; bất kỳ phương án thay thế nào cũng xác nhận được mũi; yêu cầu cùng sản phẩm là quy tắc cấp bệnh và cảnh báo đổi loại cần ≥ 2 sản phẩm đã biết; giải thích phác đồ an toàn nhất liệt kê từng phác đồ; kiểm tra đầu vào tách riêng, `INVALID_DATE` | — | F04.3, F05.1, F05.2, F05.3, F05.10, §5 |

Ghi chú:
- EQ3, EQ4 và EQ9 chỉ được giữ để truy vết: EQ3 và EQ4 được thay bằng EQ15, EQ9 được thay bằng EQ19.
- EQ21 đặt "chắc chắn đã quá tuổi" (tầng 5) trên các mặc định theo tình trạng lịch sử (tầng 6), đảo ngược thứ tự trước đây "NOT_ENTERED trước tuổi tối đa" (SAD v1.0 §5.3 bước 7; nay là SAD §5.3 bước 10).
- F13.2 được thêm các ca kiểm thử liệt kê cho EQ1–EQ25, và hai ca hiện có được viết lại ("≥ 1 mũi đã tiêm" cho ✅ "Đang theo phác đồ"; các bệnh quá tuổi cho người lớn 45 tuổi).

