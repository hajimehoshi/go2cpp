// SPDX-License-Identifier: Apache-2.0

package gowasm2cpp

import (
	"os"
	"path/filepath"
	"text/template"
)

func writeGame(dir string, incpath string, namespace string) error {
	{
		f, err := os.Create(filepath.Join(dir, "game.h"))
		if err != nil {
			return err
		}
		defer f.Close()

		if err := gameHTmpl.Execute(f, struct {
			IncludeGuard string
			IncludePath  string
			Namespace    string
		}{
			IncludeGuard: includeGuard(namespace) + "_GAME_H",
			IncludePath:  incpath,
			Namespace:    namespace,
		}); err != nil {
			return err
		}
	}
	{
		f, err := os.Create(filepath.Join(dir, "game.cpp"))
		if err != nil {
			return err
		}
		defer f.Close()

		if err := gameCppTmpl.Execute(f, struct {
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

var gameHTmpl = template.Must(template.New("game.h").Parse(`// Code generated by go2cpp. DO NOT EDIT.

#ifndef {{.IncludeGuard}}
#define {{.IncludeGuard}}

#include <functional>
#include <memory>
#include "{{.IncludePath}}go.h"

namespace {{.Namespace}} {

class Game {
public:
  class Driver {
  public:
    virtual ~Driver();
    virtual bool Init() = 0;
    virtual void Update(std::function<void()> f) = 0;
    virtual int GetScreenWidth() = 0;
    virtual int GetScreenHeight() = 0;
    virtual double GetDevicePixelRatio() = 0;
    virtual void* GetOpenGLFunction(const char* name) = 0;
    virtual int GetTouchCount() = 0;
    virtual void GetTouchPosition(int index, int* id, int* x, int* y) = 0;
  };

  Game(std::unique_ptr<Driver> driver);

  int Run();

private:
  void Update(Value f);

  std::unique_ptr<Driver> driver_;
};

}

#endif  // {{.IncludeGuard}}
`))

var gameCppTmpl = template.Must(template.New("game.cpp").Parse(`// Code generated by go2cpp. DO NOT EDIT.

#include "{{.IncludePath}}game.h"

#include "{{.IncludePath}}gl.h"

namespace {{.Namespace}} {

Game::Driver::~Driver() = default;

Game::Game(std::unique_ptr<Driver> driver)
  : driver_(std::move(driver)) {
}

int Game::Run() {
  if (!driver_->Init()) {
    return EXIT_FAILURE;
  }

  auto& global = Value::Global().ToObject();
  auto go2cpp = std::make_shared<DictionaryValues>();
  global.Set("go2cpp", Value{go2cpp});

  auto gl = std::make_shared<GL>([this](const char* name) -> void* {
    return driver_->GetOpenGLFunction(name);
  });
  go2cpp->Set("gl", Value{gl});

  go2cpp->Set("screenWidth",
      Value{static_cast<double>(driver_->GetScreenWidth())});
  go2cpp->Set("screenHeight",
      Value{static_cast<double>(driver_->GetScreenHeight())});
  go2cpp->Set("devicePixelRatio", Value{driver_->GetDevicePixelRatio()});
  go2cpp->Set("touchCount", Value{0.0});
  go2cpp->Set("getTouchPositionId", Value{std::make_shared<Function>(
    [this](Value self, std::vector<Value> args) -> Value {
      int idx = static_cast<int>(args[0].ToNumber());
      int id;
      driver_->GetTouchPosition(idx, &id, nullptr, nullptr);
      return Value{static_cast<double>(id)};
    })});
  go2cpp->Set("getTouchPositionX", Value{std::make_shared<Function>(
    [this](Value self, std::vector<Value> args) -> Value {
      int idx = static_cast<int>(args[0].ToNumber());
      int x;
      driver_->GetTouchPosition(idx, nullptr, &x, nullptr);
      return Value{static_cast<double>(x)};
    })});
  go2cpp->Set("getTouchPositionY", Value{std::make_shared<Function>(
    [this](Value self, std::vector<Value> args) -> Value {
      int idx = static_cast<int>(args[0].ToNumber());
      int y;
      driver_->GetTouchPosition(idx, nullptr, nullptr, &y);
      return Value{static_cast<double>(y)};
    })});

  Go go;
  global.Set("requestAnimationFrame",
             Value{std::make_shared<Function>(
                 [this, &go](Value self, std::vector<Value> args) -> Value {
                   Value f = args[0];
                   go.EnqueueTask([this, f]() {
                     driver_->Update([this, f]() mutable {
                       Update(f);
                     });
                   });
                   return Value{};
                 })});
  return go.Run();
}

void Game::Update(Value f) {
  auto& global = Value::Global().ToObject();
  auto& go2cpp = global.Get("go2cpp").ToObject();
  go2cpp.Set("touchCount", Value{static_cast<double>(driver_->GetTouchCount())});

  f.ToObject().Invoke(Value{}, {});
}

}
`))
