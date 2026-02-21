# Golang Custom Validation & regexp.MustCompile Performance Tuning

เกริ่นนำ บทความนี้เป็นการหยิบตัวอย่างง่ายๆแต่เจอได้บ่อยๆที่พึ่งพบจากการ review code(golang) มาเล่าให้ฟังกันเพื่อเป็นไอเดียกันครับ

ในการพัฒนา API ด้วย Go ไลบรารียอดฮิตที่แทบทุกโปรเจกต์ไม่ว่าจะเป็น Web API หรือ CLI Tool ก็ตามคงหนีไม่พ้น `github.com/go-playground/validator` ซึ่งช่วยให้เราตรวจสอบข้อมูล (Validate) ฟีลข้อมูลเช่นจาก Request ที่เข้ามาผ่าน Struct Tag ได้อย่างง่ายดาย  

![Golang Custom Validator](resources/golang_custom_validator.png)

แต่รู้หรือไม่ว่า... หากเราสร้าง Custom Validation ที่ต้องใช้ Regular Expression (regexp) แบบไม่ระวัง เราอาจกำลังสร้าง "คอขวด" ด้าน Performance ให้กับระบบโดยไม่รู้ตัว!

เราจะมาดูวิธีการสร้าง Custom Validator ที่เจอข้อผิดพลาดที่พบได้บ่อย (Pitfalls) และพิสูจน์ความเร็วให้เห็นกันชัดๆ ด้วย Benchmark ครับ

---

## ปูพื้นฐาน: การใช้งาน Custom Validation ด้วย Struct Tag

สมมติว่าเรามี API สำหรับสร้างสินค้าใหม่ และเราต้องการบังคับว่าฟิลด์ ProductID ต้องขึ้นต้นด้วยคำว่า PROD- ตามด้วยตัวเลข 4 หลักเท่านั้น (เช่น PROD-1234)

เราสามารถสร้าง Tag พิเศษของเราเอง เช่น validate:"product_id" และนำไปผูกกับ Struct ได้แบบนี้

```go
package main

import (
    "fmt"
    "regexp"

    "github.com/go-playground/validator/v10"
)

type CreateProductRequest struct {
  Name      string `validate:"required"`
  ProductID string `validate:"required,product_id"` // เรียกใช้ Custom Tag ตรงนี้
}

func main() {
  v := validator.New()

  // ⚠️ จำโค้ดส่วนนี้ไว้ให้ดี นี่คือจุดเริ่มต้นของปัญหา! ⚠️
  v.RegisterValidation("product_id", func(fl validator.FieldLevel) bool {
    re := regexp.MustCompile(`^PROD-\d{4}$`)
    return re.MatchString(fl.Field().String())
  })

  req := CreateProductRequest{
    Name:      "Mechanical Keyboard",
    ProductID: "PROD-9999",
  }

  if err := v.Struct(req); err != nil {
    fmt.Println("Validation Failed:", err)
  } else {
    fmt.Println("Validation Passed! 🎉")
  }
}
```

ดูเผินๆ โค้ดด้านบนทำงานได้ถูกต้องสมบูรณ์แบบ... แต่ในแง่ของ Performance แล้วมันอาจจะทำงานได้ช้ากว่าที่ควรเป็นมากเมื่อนำไปใช้จริง!

## The Pitfall: ทำไมโค้ดด้านบนถึงมีปัญหา?

ปัญหาไม่ได้อยู่ที่ตัวไลบรารี validator แต่อยู่ที่พฤติกรรมของ regexp.MustCompile() ครับ

ฟังก์ชันนี้จะทำการ Parse ตัว String Pattern ที่เราส่งเข้าไป แล้วสร้างเป็น State Machine เพื่อเตรียมไว้ค้นหา กระบวนการนี้ กินทรัพยากร CPU และ Memory สูงมาก

เมื่อเราเอา MustCompile ซึ่งเป็น function ที่ทำงานได้ช้าไปวางไว้ "ภายใน" Anonymous Function ของ RegisterValidation แปลว่า ทุกๆ ครั้งที่มี Request เข้ามา โปรแกรมจะสั่ง Compile Regex ใหม่ซ้ำๆ ทุกครั้ง! ถ้าระบบมี 1,000 Request ต่อวินาที ก็คือ Compile ใหม่ 1,000 ครั้ง ซึ่งเป็นการทิ้งทรัพยากรไปฟรีๆ

กฎการใช้งาน Regex บนภาษา Go อย่างถูกต้องคือ "เราควร Compile มันแค่ครั้งเดียว" ครับ เรามาดู 3 วิธีแก้ปัญหานี้กัน

## วิธีแก้ไข

วิธีที่ 1: Package Level Variable (วิธีมาตรฐาน) ✅
ดึงตัวแปรออกมาไว้ระดับ Package ฟังก์ชัน MustCompile จะทำงานแค่ ครั้งเดียว ตอนที่โปรแกรมเริ่มต้น (Startup)

```go
// Compile ครั้งเดียวตอนเริ่มโปรแกรม
  var productIDRegex = regexp.MustCompile(`^PROD-\d{4}$`)

  func SetupValidator() *validator.Validate {
    v := validator.New()
    v.RegisterValidation("product_id", func(fl validator.FieldLevel) bool {
      return productIDRegex.MatchString(fl.Field().String())
    })
    return v
}
```

วิธีที่ 2: The Closure (ซ่อนตัวแปรไว้ใน Constructor) ✅
ถ้าไม่อยากให้ตัวแปรไปอยู่ระดับ Global เราสามารถดึงขึ้นมาไว้ในฟังก์ชัน Setup ได้ วิธีนี้ใช้ระบบ Closure ของ Go ในการจดจำค่าไว้

```go
func SetupValidator() *validator.Validate {
  v := validator.New()
  
  // Compile 1 ครั้ง ตอนเรียก SetupValidator
  re := regexp.MustCompile(`^PROD-\d{4}$`)
  
  v.RegisterValidation("product_id", func(fl validator.FieldLevel) bool {
    // ฟังก์ชันด้านในจะจำค่า re เอาไว้ใช้งาน
    return re.MatchString(fl.Field().String())
  })
  return v
}
```

วิธีที่ 3: sync.Once (Lazy Initialization สำหรับระบบใหญ่) 🔥
ถ้าคุณมี Custom Regex Validator เป็นร้อยๆ ตัว การใช้ วิธีที่ 1 และ 2 อาจทำให้ตอน Boot Server (Startup Time) ช้าลงได้ sync.Once จึงเข้ามาตอบโจทย์นี้ เพราะมันจะสั่ง Compile "เมื่อมีการเรียกใช้งานครั้งแรกเท่านั้น" (Lazy Load)

```go
import "sync"

func newSyncOnceValidator() func(string) bool {
  var (
    once sync.Once
    re   *regexp.Regexp
  )

  return func(val string) bool {
    // โค้ดใน once.Do จะทำงานแค่ "ครั้งแรกครั้งเดียว" ตลอดอายุของโปรแกรม
    once.Do(func() {
      re = regexp.MustCompile(`^PROD-\d{4}$`)
    })
    return re.MatchString(val)
  }
}
```

## วัดผลการทำงานเปรียบเทียบความเร็ว ด้วยการ Benchmark

เรามาเขียน Benchmark เพื่อเปรียบเทียบ "ฟังก์ชันตรวจสอบเนื้อใน" ของทั้ง 4 แบบกันครับ (จำลองเฉพาะฟังก์ชันตรวจสอบเพื่อความชัดเจน)

Step 1: สร้างโปรเจกต์

```bash
mkdir go-custom-validate
cd go-custom-validate
go mod init go-custom-validate
```

Step 2: สร้างไฟล์ main_test.go และเขียนโค้ดสำหรับ Benchmark

```go
package main

import (
  "regexp"
  "sync"
  "testing"
)

// ❌ 1. แบบที่มีปัญหา (Compile ทุกครั้ง)
func badValidator(val string) bool {
  re := regexp.MustCompile(`^PROD-\d{4}$`)
  return re.MatchString(val)
  }

// ✅ 2. แบบ Package Variable
var compiledRegex = regexp.MustCompile(`^PROD-\d{4}$`)
  func pkgValidator(val string) bool {
  return compiledRegex.MatchString(val)
}

// ✅ 3. แบบ Closure
func newClosureValidator() func(string) bool {
  re := regexp.MustCompile(`^PROD-\d{4}$`)
  return func(val string) bool { return re.MatchString(val) }
}

// ✅ 4. แบบ sync.Once
func newSyncOnceValidator() func(string) bool {
  var once sync.Once
  var re *regexp.Regexp
  return func(val string) bool {
    once.Do(func() { re = regexp.MustCompile(`^PROD-\d{4}$`) })
    return re.MatchString(val)
  }
}

// --- โค้ดสำหรับ Benchmark ---

func BenchmarkBadValidator(b *testing.B) {
  for i := 0; i < b.N; i++ { badValidator("PROD-1234") }
}
func BenchmarkPkgValidator(b *testing.B) {
  for i := 0; i < b.N; i++ { pkgValidator("PROD-1234") }
}
func BenchmarkClosureValidator(b *testing.B) {
  v := newClosureValidator()
  b.ResetTimer()
  for i := 0; i < b.N; i++ { v("PROD-1234") }
}
func BenchmarkSyncOnceValidator(b *testing.B) {
  v := newSyncOnceValidator()
  b.ResetTimer()
  for i := 0; i < b.N; i++ { v("PROD-1234") }
}
```

Step 3: รัน Benchmark พร้อมดู Memory

```bash
go test -bench=. -benchmem  # ถ้า git clone project นี้ถ้ามารถใช้ make bench ได้เลย
```

เมื่อรันเสร็จ คุณจะได้ผลลัพธ์หน้าตาคล้ายๆ ตารางแบบนี้ครับ

```bash
go test -bench=. -benchmem
goos: darwin
goarch: arm64
pkg: go-custam-validate
cpu: Apple M4 Pro
BenchmarkBadValidator-12                  461996                2246 ns/op            4530 B/op        59 allocs/op
BenchmarkPkgValidator-12                21121894                60.37 ns/op            0 B/op          0 allocs/op
BenchmarkClosureValidator-12            21938026                55.90 ns/op            0 B/op          0 allocs/op
BenchmarkSyncOnceValidator-12           20153840                57.84 ns/op            0 B/op          0 allocs/op
PASS
ok      go-custam-validate      5.601s
```

## อ่านผล Benchmark แบบ Deep Dive 📊

มาเจาะลึกตัวเลขแต่ละคอลัมน์กัน

Note: ในการรัน benchmark แต่ละรอบค่าจะได้ไม่เท่าเดิมแต่ละรอบ เพราะขึ้นอยู่กับสภาพแวดล้อมของเครื่องที่รันในขณะนั้นด้วย แต่โดยรวมแล้วเพียงพอต่อการวิเคราะห์แนวโน้มของแต่ละฟังก์ชันได้ครับ

- Column 1: ชื่อฟังก์ชัน (Benchmark Name)

เช่น BenchmarkBadValidator-12
ค่าตัวเลข -12 ด้านหลังคือค่า GOMAXPROCS หรือจำนวน CPU Cores สูงสุดที่โปรแกรมใช้ในการรันเทสนี้  
ในกรณีนี้อาจจะได้ว่าเครื่องที่ทำงานมี 12 Cores หรืออาจจะไปกำหนดให้ ใช้แค่ 12 Cores ในการรันเทสก็ได้

- Column 2: จำนวนรอบ (Number of Iterations / b.N)

เช่น 461996 เทียบกับ 20153840  
คือจำนวน "รอบ" ที่ Go สามารถรันลูป `for i := 0; i < b.N; i++` ได้ภายในเวลาที่กำหนด (ค่าเริ่มต้นคือ 1 วินาที)  
**ยิ่งมากยิ่งดี แปลว่าใน 1 วินาทีมันทำงานได้หลายรอบ (รันได้เร็วกว่า)**

- Column 3: เวลาต่อ 1 รอบ (ns/op - Nanoseconds per operation)

เช่น 2246 ns/op  
คือเวลาเฉลี่ยที่ใช้ในการทำงาน "1 รอบ" (หน่วยเป็นนาโนวินาที)  
**ยิ่งน้อยแปลว่า ทำงานเสร็จไวกว่า**

- Column 4: ปริมาณ Memory ที่จองต่อ 1 รอบ (B/op - Bytes per operation)

เช่น 4530 B/op เทียบกับ 0 B/op  
คือค่านี้แสดงให้เห็นว่าเฉลี่ยแล้วในการทำงาน 1 รอบ โค้ดของเราต้องขอจองพื้นที่ RAM ไปกี่ Byte 
**ยิ่งตัวเลขนี้ "น้อย" ยิ่งดี ถ้าเป็น 0 ได้คือสุดยอด** เพราะแปลว่ารันไปยาวๆแล้วไม่ต้องมีการจอง Memory เพิ่มเลยแม้แต่ Byte เดียว

- Column 5: จำนวนครั้งที่จอง Memory ต่อรอบ (allocs/op - Allocations per op)

เช่น 59 allocs/op เทียบกับ 0 allocs/op  
คือจำนวน "ครั้ง" (ไม่ใช่ขนาด) ที่โค้ดของเราสั่งขอกล่อง Memory ใบใหม่จากระบบ  
**ยิ่ง"น้อย" ยิ่งดี** เพราะ Garbage Collector (GC) ไม่ต้องมาคอยตามเก็บกวาดทีหลัง ทำให้ CPU ไม่ต้องทำงานหนัก

## สรุป

The Pitfall (BadValidator): ใช้เวลาไปถึง ~2,246 ns ต่อรอบ! แถมใน 1 รอบ ยังต้องจอง Memory ใหม่ถึง 56 ครั้ง รวมเป็น 4,530 Bytes!  
ถ้า service ที่ใช้งาน code ดังกล่าวเป็น service ที่ถูกเรียกเยอะๆ Server จะต้องทำงานหนักมากในการ Compile Regex ซ้ำๆ และยังต้องคอยเก็บกวาด Memory ที่ถูกจองไปเรื่อยๆ ทำให้ระบบช้าและไม่เสถียรได้ในระยะยาว

Solutions ที่สามารถใช้แก้ปัญหาทั้ง 3 แบบ ได้แก่ PkgValidator, ClosureValidator และ SyncOnceValidator ใช้เวลาประมาณ ~55-60 ns (เร็วกว่าประมาณ 40 เท่า!!) และไม่กิน Memory เพิ่มเติมเลยแม้แต่ Byte เดียว (0 allocs/op)

รายละเอียดในการเขียน Go เล็กๆ น้อยๆ อย่างตำแหน่งการวางโค้ดแค่บรรทัดเดียวเอาไว้ใน callback function ก็สามารถสร้างผลกระทบต่อระบบได้อย่างมหาศาลครับ ครั้งหน้าถ้าต้องใช้ regexp หรือฟังก์ชันที่ทำงานช้าเช่นพวกที่ลงท้ายด้วยคำว่า Parse, Compile, Build อย่าลืมตรวจสอบให้ดีว่าจำเป็นจริงๆและวางถูกตำแหน่งแล้วใช่มั้ย หวังว่าบทความนี้จะช่วยให้ทุกคนจัดการกับคอขวดใน Go ได้อย่างมั่นใจมากขึ้นครับ 💻✨
