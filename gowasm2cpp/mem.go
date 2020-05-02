// SPDX-License-Identifier: Apache-2.0

package gowasm2cpp

import (
	"os"
	"path/filepath"
	"text/template"
)

type wasmData struct {
	Offset int
	Data   []byte
}

func writeMemCS(dir string, namespace string, initPageNum int, data []wasmData) error {
	{
		f, err := os.Create(filepath.Join(dir, "mem.h"))
		if err != nil {
			return err
		}
		defer f.Close()

		if err := memHTmpl.Execute(f, struct {
			IncludeGuard string
			Namespace    string
		}{
			IncludeGuard: includeGuard(namespace) + "_MEM_H",
			Namespace:    namespace,
		}); err != nil {
			return err
		}
	}
	{
		f, err := os.Create(filepath.Join(dir, "mem.cpp"))
		if err != nil {
			return err
		}
		defer f.Close()

		if err := memCppTmpl.Execute(f, struct {
			Namespace    string
			InitPageNum  int
			Data         []wasmData
		}{
			Namespace:   namespace,
			InitPageNum: initPageNum,
			Data:        data,
		}); err != nil {
			return err
		}
	}
	return nil
}

var memHTmpl = template.Must(template.New("mem.h").Parse(`// Code generated by go2cpp. DO NOT EDIT.

#ifndef {{.IncludeGuard}}
#define {{.IncludeGuard}}

#include <cstdint>
#include <string>
#include <vector>

namespace {{.Namespace}} {

// TODO: Replace this with std::span in the C++20 era.
class BytesSegment {
public:
  using size_type = std::vector<uint8_t>::size_type;
  using const_iterator = std::vector<uint8_t>::const_iterator;

  BytesSegment(const std::vector<uint8_t>& bytes, size_type offset, size_type length);

  uint8_t operator[](size_type n) const;
  size_type size() const;
  const_iterator begin() const;
  const_iterator end() const;

private:
  const std::vector<uint8_t>& bytes_;
  const size_type offset_ = 0;
  const size_type length_ = 0;
};

class Mem {
public:
  static const int32_t kPageSize = 64 * 1024;

  Mem();

  int32_t GetSize() const;
  int32_t Grow(int32_t delta);

  int8_t LoadInt8(int32_t addr) const;
  uint8_t LoadUInt8(int32_t addr) const;
  int16_t LoadInt16(int32_t addr) const;
  uint16_t LoadUInt16(int32_t addr) const;
  int32_t LoadInt32(int32_t addr) const;
  uint32_t LoadUInt32(int32_t addr) const;
  int64_t LoadInt64(int32_t addr) const;
  float LoadFloat32(int32_t addr) const;
  double LoadFloat64(int32_t addr) const;

  void StoreInt8(int32_t addr, int8_t val);
  void StoreInt16(int32_t addr, int16_t val);
  void StoreInt32(int32_t addr, int32_t val);
  void StoreInt64(int32_t addr, int64_t val);
  void StoreFloat32(int32_t addr, float val);
  void StoreFloat64(int32_t addr, double val);
  void StoreBytes(int32_t addr, const std::vector<uint8_t>& bytes);

  BytesSegment LoadSlice(int32_t addr) const;
  BytesSegment LoadSliceDirectly(int64_t array, int32_t len) const;
  std::string LoadString(int32_t addr) const;

private:
  Mem(const Mem&) = delete;
  Mem& operator=(const Mem&) = delete;

  std::vector<uint8_t> bytes_;
};

}

#endif  // {{.IncludeGuard}}
`))

var memCppTmpl = template.Must(template.New("mem.cpp").Parse(`// Code generated by go2cpp. DO NOT EDIT.

#include "autogen/mem.h"

namespace {{.Namespace}} {

BytesSegment::BytesSegment(const std::vector<uint8_t>& bytes, size_type offset, size_type length)
    : bytes_(bytes),
      offset_(offset),
      length_(length) {
}

uint8_t BytesSegment::operator[](size_type n) const {
  return bytes_[n + offset_];
}

BytesSegment::size_type BytesSegment::size() const {
  return length_;
}

BytesSegment::const_iterator BytesSegment::begin() const {
  return bytes_.begin() + offset_;
}

BytesSegment::const_iterator BytesSegment::end() const {
  return bytes_.begin() + offset_ + length_;
}

Mem::Mem()
    : bytes_({{.InitPageNum}} * kPageSize) {
{{range $value := .Data}}  {
    uint8_t arr[] = { {{- range $value2 := $value.Data}}{{$value2}},{{end -}} };
    std::copy(std::begin(arr), std::end(arr), bytes_.begin() + {{$value.Offset}});
  }
{{end}}
}

int32_t Mem::GetSize() const {
  return bytes_.size() / kPageSize;
}

int32_t Mem::Grow(int32_t delta) {
  int prev_size = GetSize();
  bytes_.resize((prev_size + delta) * kPageSize);
  return prev_size;
}

int8_t Mem::LoadInt8(int32_t addr) const {
  return static_cast<int8_t>(bytes_[addr]);
}

uint8_t Mem::LoadUInt8(int32_t addr) const {
  return bytes_[addr];
}

int16_t Mem::LoadInt16(int32_t addr) const {
  return *(reinterpret_cast<const int16_t*>(&bytes_[addr]));
}

uint16_t Mem::LoadUInt16(int32_t addr) const {
  return *(reinterpret_cast<const uint16_t*>(&bytes_[addr]));
}

int32_t Mem::LoadInt32(int32_t addr) const {
  return *(reinterpret_cast<const int32_t*>(&bytes_[addr]));
}

uint32_t Mem::LoadUInt32(int32_t addr) const {
  return *(reinterpret_cast<const uint32_t*>(&bytes_[addr]));
}

int64_t Mem::LoadInt64(int32_t addr) const {
  return *(reinterpret_cast<const int64_t*>(&bytes_[addr]));
}

float Mem::LoadFloat32(int32_t addr) const {
  return *(reinterpret_cast<const float*>(&bytes_[addr]));
}

double Mem::LoadFloat64(int32_t addr) const {
  return *(reinterpret_cast<const double*>(&bytes_[addr]));
}

void Mem::StoreInt8(int32_t addr, int8_t val) {
  bytes_[addr] = static_cast<uint8_t>(val);
}

void Mem::StoreInt16(int32_t addr, int16_t val) {
  *(reinterpret_cast<int16_t*>(&bytes_[addr])) = val;
}

void Mem::StoreInt32(int32_t addr, int32_t val) {
  *(reinterpret_cast<int32_t*>(&bytes_[addr])) = val;
}

void Mem::StoreInt64(int32_t addr, int64_t val) {
  *(reinterpret_cast<int64_t*>(&bytes_[addr])) = val;
}

void Mem::StoreFloat32(int32_t addr, float val) {
  *(reinterpret_cast<float*>(&bytes_[addr])) = val;
}

void Mem::StoreFloat64(int32_t addr, double val) {
  *(reinterpret_cast<double*>(&bytes_[addr])) = val;
}

void Mem::StoreBytes(int32_t addr, const std::vector<uint8_t>& bytes) {
  std::copy(bytes.begin(), bytes.end(), bytes_.begin() + addr);
}

BytesSegment Mem::LoadSlice(int32_t addr) const {
  int64_t array = LoadInt64(addr);
  int64_t len = LoadInt64(addr + 8);
  return BytesSegment{bytes_, static_cast<BytesSegment::size_type>(array), static_cast<BytesSegment::size_type>(len)};
}

BytesSegment Mem::LoadSliceDirectly(int64_t array, int32_t len) const {
  return BytesSegment{bytes_, static_cast<BytesSegment::size_type>(array), static_cast<BytesSegment::size_type>(len)};
}

std::string Mem::LoadString(int32_t addr) const {
  int64_t saddr = LoadInt64(addr);
  int64_t len = LoadInt64(addr + 8);
  return std::string{bytes_.begin() + saddr, bytes_.begin() + saddr + len};
}

}
`))
