// PROTOTYPE — throwaway. The five hard cases, defined once in Go so native golden output
// and the browser (WASM) run exactly the same inputs.
package scenarios

import (
	_ "embed"
	"encoding/json"

	e "vaxbook.local/engine-prototype/engine"
)

//go:embed ruleset.json
var RuleSetJSON []byte

type Step struct {
	Label  string  `json:"label"`
	Expect string  `json:"expect"`
	Input  e.Input `json:"input"`
}

type Scenario struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Question string `json:"question"`
	Steps    []Step `json:"steps"`
}

func day(v string) e.FuzzyDate   { return e.FuzzyDate{Precision: e.PDay, Value: v} }
func month(v string) e.FuzzyDate { return e.FuzzyDate{Precision: e.PMonth, Value: v} }
func year(v string) e.FuzzyDate  { return e.FuzzyDate{Precision: e.PYear, Value: v} }
func d(v string) e.Date {
	x, err := e.ParseDate(v)
	if err != nil {
		panic(err)
	}
	return x
}

type builder struct {
	in    e.Input
	steps []Step
}

func (b *builder) step(label, expect string, mut func(in *e.Input)) {
	mut(&b.in)
	raw, _ := json.Marshal(b.in)
	var cp e.Input
	_ = json.Unmarshal(raw, &cp)
	b.steps = append(b.steps, Step{Label: label, Expect: expect, Input: cp})
}

func findDose(in *e.Input, id string) *e.DoseRecord {
	for i := range in.Doses {
		if in.Doses[i].ID == id {
			return &in.Doses[i]
		}
	}
	panic(id)
}

func All() []Scenario {
	var out []Scenario

	// 1. Safest series for an unknown Rota product (F05.3).
	{
		b := &builder{in: e.Input{Today: d("2026-10-05"), Profile: e.Profile{Birth: day("2026-07-01"), HistoryStatus: "complete", HistoryStatusAt: d("2026-10-05")}}}
		b.step("Bé sinh 01/07/2026. Nhập 1 mũi Rota ngày 26/08/2026, không nhớ tên vaccine. Hôm nay 05/10/2026.",
			"Rota: 🟠 mũi 2, áp dụng phác đồ an toàn nhất (3 mũi, mốc muộn nhất).",
			func(in *e.Input) {
				in.Doses = append(in.Doses, e.DoseRecord{ID: "r1", Diseases: []string{"rota"}, Date: day("2026-08-26")})
			})
		b.step("Thời gian trôi tới 06/01/2027 (bé 27 tuần), vẫn chưa uống mũi 2.",
			"Rota: ⚪ — Rotarix & Rotavin đã quá tuổi hoàn thành, RotaTeq chưa → kết luận khác nhau → kiểm tra tên vaccine.",
			func(in *e.Input) { in.Today = d("2027-01-06") })
		b.step("Tìm thấy sổ: mũi 1 là RotaTeq.",
			"Rota: 🔴 mũi 2 (RotaTeq còn trong tuổi tới 32 tuần).",
			func(in *e.Input) { findDose(in, "r1").Product = "rotateq" })
		b.step("Nhầm — sổ ghi Rotarix.",
			"Rota: khối \"Không còn trong độ tuổi tiêm\" (Rotarix phải xong trước 24 tuần).",
			func(in *e.Input) { findDose(in, "r1").Product = "rotarix" })
		out = append(out, Scenario{ID: "safest", Title: "1 · Rota không rõ sản phẩm",
			Question: "Phác đồ an toàn nhất có gộp đúng theo từng chiều, và khi các sản phẩm khả dĩ bất đồng về tuổi tối đa thì có ra ⚪ thay vì báo quá tuổi?",
			Steps: b.steps})
	}

	// 2. Overlapping estimated dates (F04.2, F05.10).
	{
		b := &builder{in: e.Input{Today: d("2026-11-10"), Profile: e.Profile{Birth: day("2023-03-10"), HistoryStatus: "complete", HistoryStatusAt: d("2026-11-10")}}}
		b.step("Bé sinh 10/03/2023. Nhập mũi ho gà: chỉ nhớ năm 2023.",
			"Ho gà: ⚪ — năm 2023 bao cả những ngày trước 6 tuần tuổi → mũi có thể không hợp lệ.",
			func(in *e.Input) {
				in.Doses = append(in.Doses, e.DoseRecord{ID: "p1", Diseases: []string{"pertussis"}, Product: "hexaxim", Date: year("2023")})
			})
		b.step("Nhập thêm mũi ho gà: tháng 06/2023.",
			"Ho gà: ⚪ — hai khoảng ngày chồng lấn, không xếp được thứ tự.",
			func(in *e.Input) {
				in.Doses = append(in.Doses, e.DoseRecord{ID: "p2", Diseases: []string{"pertussis"}, Product: "hexaxim", Date: month("2023-06")})
			})
		b.step("Nhập số thứ tự: mũi \"năm 2023\" là mũi 1, mũi \"tháng 6\" là mũi 2.",
			"Ho gà: hết lỗi chồng lấn, nhưng vẫn ⚪ vì mũi 1 (cả năm 2023) có thể trước 6 tuần tuổi.",
			func(in *e.Input) {
				findDose(in, "p1").DoseNumber = map[string]int{"pertussis": 1}
				findDose(in, "p2").DoseNumber = map[string]int{"pertussis": 2}
			})
		b.step("Nhớ ra: mũi 1 vào tháng 05/2023.",
			"Ho gà: vẫn ⚪ — tháng 5 và tháng 6 có thể chỉ cách nhau 1 ngày (< 4 tuần).",
			func(in *e.Input) { findDose(in, "p1").Date = month("2023-05") })
		b.step("Xem sổ, nhập đủ ngày: 10/05/2023 và 12/06/2023.",
			"Ho gà: 🔴 mũi 3 cần tiêm bù, kèm ghi chú lịch 2-3-4 hoặc 2-4-6.",
			func(in *e.Input) {
				findDose(in, "p1").Date = day("2023-05-10")
				findDose(in, "p2").Date = day("2023-06-12")
			})
		out = append(out, Scenario{ID: "overlap", Title: "2 · Ngày ước lượng chồng lấn",
			Question: "Quy tắc \"chồng lấn → ⚪, dùng số mũi user nhập nếu có\" và \"mũi ước lượng kiểm tra ở cả hai đầu\" cho trải nghiệm thế nào khi user chỉ nhớ tháng?",
			Steps: b.steps})
	}

	// 3. Gardasil 9 with an estimated age at start (F05.5).
	{
		b := &builder{in: e.Input{Today: d("2026-11-10"), Profile: e.Profile{Birth: year("2011"), Sex: "F", HistoryStatus: "complete", HistoryStatusAt: d("2026-11-10")}}}
		b.step("Nữ, chỉ nhớ năm sinh 2011. Mũi 1 Gardasil 9 vào tháng 01/2026.",
			"HPV: tuổi lúc tiêm mũi 1 nằm trong 14–15 → chọn phác đồ 3 mũi, nhãn ước lượng; mũi 2 🔴.",
			func(in *e.Input) {
				in.Doses = append(in.Doses, e.DoseRecord{ID: "g1", Diseases: []string{"hpv"}, Product: "gardasil9", Date: month("2026-01")})
			})
		b.step("Nhập ngày sinh đầy đủ: 20/09/2011.",
			"HPV: 14 tuổi khi tiêm mũi 1 → phác đồ 2 mũi; mũi 2 đến hạn ~07/2026 → 🔴.",
			func(in *e.Input) { in.Profile.Birth = day("2011-09-20") })
		b.step("Sửa lại: sinh 05/12/2010.",
			"HPV: 15 tuổi khi tiêm mũi 1 → phác đồ 3 mũi.",
			func(in *e.Input) { in.Profile.Birth = day("2010-12-05") })
		out = append(out, Scenario{ID: "gardasil", Title: "3 · Gardasil 9 tuổi ước lượng",
			Question: "Khi tuổi lúc bắt đầu là một khoảng vắt qua ranh giới 15 tuổi, engine có chọn phác đồ nhiều mũi hơn (khác với \"phác đồ an toàn nhất\" giữa các sản phẩm)?",
			Steps: b.steps})
	}

	// 4. Alternative plans 2-3-4 vs 2-4-6 (F05.4).
	{
		b := &builder{in: e.Input{Today: d("2026-09-01"), Profile: e.Profile{Birth: day("2026-06-15"), HistoryStatus: "complete", HistoryStatusAt: d("2026-09-01")}}}
		b.step("Bé sinh 15/06/2026, mũi 6in1 đầu ngày 15/08/2026. Hôm nay 01/09/2026.",
			"Ho gà: 🟡 mũi 2 (sớm nhất 12/09, đến hạn 15/09 theo 2-3-4, quá hạn 15/10 theo 2-4-6).",
			func(in *e.Input) {
				in.Doses = append(in.Doses, e.DoseRecord{ID: "h1", Diseases: []string{"pertussis"}, Product: "hexaxim", Date: day("2026-08-15")})
			})
		b.step("Hôm nay 20/09/2026.", "Ho gà: 🟠 đến hạn.", func(in *e.Input) { in.Today = d("2026-09-20") })
		b.step("Hôm nay 10/10/2026.",
			"Ho gà: vẫn 🟠 — nếu chỉ có lịch 2-3-4 thì đã 🔴 từ 15/09.",
			func(in *e.Input) { in.Today = d("2026-10-10") })
		b.step("Hôm nay 16/10/2026.", "Ho gà: 🔴 (qua mốc muộn nhất 15/10).", func(in *e.Input) { in.Today = d("2026-10-16") })
		out = append(out, Scenario{ID: "alternatives", Title: "4 · Lịch 2-3-4 hay 2-4-6",
			Question: "Ghép hai phương án (đến hạn theo phương án sớm, quá hạn theo phương án muộn) có tạo ra khoảng 🟠 hợp lý và không tự suy phương án?",
			Steps: b.steps})
	}

	// 5. "Chưa nhập xong" with milestones before/after the moment it was chosen (F03.6).
	{
		b := &builder{in: e.Input{Today: d("2026-11-10"), Profile: e.Profile{Birth: day("2026-05-10"), HistoryStatus: "incomplete", HistoryStatusAt: d("2026-11-10")}}}
		b.step("Bé sinh 10/05/2026 (6 tháng). Chọn \"Đã có mũi tiêm, tôi sẽ nhập\" hôm nay 10/11/2026, chưa nhập mũi nào.",
			"Rota, Ho gà: ⚪ \"Chưa nhập\" (mốc trước hôm nay) — không có 🔴. Sởi: 🟡 (mốc 9 tháng, sau thời điểm chọn).",
			func(in *e.Input) {})
		b.step("Thời gian trôi tới 20/02/2027, chưa nhập gì thêm.",
			"Sởi: 🔴 (mốc sau thời điểm chọn → tính bình thường). Rota, Ho gà vẫn ⚪.",
			func(in *e.Input) { in.Today = d("2027-02-20") })
		b.step("Nhập mũi 6in1 đầu ngày 12/07/2026.",
			"Ho gà: mũi 2 đến hạn trước thời điểm chọn → vẫn ⚪ \"Chưa nhập\".",
			func(in *e.Input) {
				in.Doses = append(in.Doses, e.DoseRecord{ID: "h1", Diseases: []string{"pertussis"}, Product: "hexaxim", Date: day("2026-07-12")})
			})
		b.step("Bấm \"Tôi đã nhập xong\".",
			"Rota: khối \"Không còn trong độ tuổi tiêm\". Ho gà: 🔴 mũi 2. Sởi: 🔴.",
			func(in *e.Input) { in.Profile.HistoryStatus = "complete" })
		out = append(out, Scenario{ID: "incomplete", Title: "5 · Hồ sơ \"Chưa nhập xong\"",
			Question: "Mốc trước thời điểm chọn → ⚪ \"Chưa nhập\", mốc sau → tính bình thường; và \"Chưa nhập\" có được ưu tiên trước \"quá tuổi\"?",
			Steps: b.steps})
	}
	return out
}
