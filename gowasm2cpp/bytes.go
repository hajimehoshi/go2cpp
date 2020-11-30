// SPDX-License-Identifier: Apache-2.0

package gowasm2cpp

import (
	"os"
	"path/filepath"
	"text/template"
)

func writeBytes(dir string, incpath string, namespace string) error {
	{
		f, err := os.Create(filepath.Join(dir, "bytes.h"))
		if err != nil {
			return err
		}
		defer f.Close()

		if err := bytesHTmpl.Execute(f, struct {
			IncludeGuard string
			IncludePath  string
			Namespace    string
		}{
			IncludeGuard: includeGuard(namespace) + "_BYTES_H",
			IncludePath:  incpath,
			Namespace:    namespace,
		}); err != nil {
			return err
		}
	}
	{
		f, err := os.Create(filepath.Join(dir, "bytes.cpp"))
		if err != nil {
			return err
		}
		defer f.Close()

		if err := bytesCppTmpl.Execute(f, struct {
			IncludePath string
			Namespace   string
		}{
			IncludePath: incpath,
			Namespace:   namespace,
		}); err != nil {
			return err
		}
	}
	return nil
}

var bytesHTmpl = template.Must(template.New("bytes.h").Parse(`// Code generated by go2cpp. DO NOT EDIT.

#ifndef {{.IncludeGuard}}
#define {{.IncludeGuard}}

#include <cstdint>
#include <vector>

namespace {{.Namespace}} {

// TODO: Replace this with std::span in the C++20 era.
class BytesSpan {
public:
  using size_type = size_t;
  using reference = uint8_t&;
  using const_reference = const uint8_t&;
  using iterator = uint8_t*;
  using const_iterator = const uint8_t*;

  BytesSpan();
  BytesSpan(uint8_t* data, size_t size);
  BytesSpan(const BytesSpan& span);
  BytesSpan& operator=(BytesSpan& span);

  reference operator[](size_type n);
  const_reference operator[](size_type n) const;
  size_type size() const;
  iterator begin();
  const_iterator begin() const;
  iterator end();
  const_iterator end() const;

  bool IsNull() const;

private:
  uint8_t* data_ = nullptr;
  size_t size_ = 0;
};

}

#endif  // {{.IncludeGuard}}
`))

var bytesCppTmpl = template.Must(template.New("bytes.cpp").Parse(`// Code generated by go2cpp. DO NOT EDIT.

#include "{{.IncludePath}}bytes.h"

namespace {{.Namespace}} {

BytesSpan::BytesSpan() = default;

BytesSpan::BytesSpan(uint8_t* data, size_t size)
    : data_(data),
      size_(size) {
}

BytesSpan::BytesSpan(const BytesSpan& span) = default;

BytesSpan& BytesSpan::operator=(BytesSpan& span) = default;

BytesSpan::reference BytesSpan::operator[](size_type n) {
  return data_[n];
}

BytesSpan::const_reference BytesSpan::operator[](size_type n) const {
  return data_[n];
}

BytesSpan::size_type BytesSpan::size() const {
  return size_;
}

BytesSpan::iterator BytesSpan::begin() {
  return data_;
}

BytesSpan::const_iterator BytesSpan::begin() const {
  return data_;
}

BytesSpan::iterator BytesSpan::end() {
  return data_ + size_;
}

BytesSpan::const_iterator BytesSpan::end() const {
  return data_ + size_;
}

bool BytesSpan::IsNull() const {
  return !data_;
}

}
`))