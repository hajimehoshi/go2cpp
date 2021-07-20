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

#include "{{.IncludePath}}go.h"

#include <cstdint>
#include <functional>
#include <memory>
#include <string>
#include <vector>

namespace {{.Namespace}} {

class Game {
public:
  struct Touch {
    int id;
    int x;
    int y;
  };

  struct Gamepad {
    int id;
    bool standard;
    int button_count;
    bool buttons[256];
    int axis_count;
    float axes[16];
  };

  class AudioPlayer {
  public:
    virtual ~AudioPlayer();
    virtual void Close(bool immediately) = 0;
    virtual double GetVolume() = 0;
    virtual void SetVolume(double volume) = 0;
    virtual void Pause() = 0;
    virtual void Play() = 0;
    virtual void Write(const uint8_t* data, int length) = 0;
    virtual size_t GetUnplayedBufferSize() = 0;
  };

  class Driver {
  public:
    virtual ~Driver();
    virtual void DebugWrite(const std::vector<uint8_t>& bytes);
    virtual bool Initialize() = 0;
    virtual bool Finalize() = 0;
    virtual void Update(std::function<void()> f) = 0;
    virtual int GetScreenWidth() = 0;
    virtual int GetScreenHeight() = 0;
    virtual double GetDevicePixelRatio() = 0;
    virtual void* GetOpenGLFunction(const char* name) = 0;
    virtual std::vector<Touch> GetTouches() = 0;
    virtual std::vector<Gamepad> GetGamepads() = 0;
    virtual std::string GetLocalStorageItem(const std::string& key) = 0;
    virtual void SetLocalStorageItem(const std::string& key, const std::string& value) = 0;
    virtual std::string GetDefaultLanguage();

    virtual void OpenAudio(int sample_rate, int channel_num, int bit_depth_in_bytes) = 0;
    virtual void CloseAudio() = 0;
    virtual std::unique_ptr<AudioPlayer> CreateAudioPlayer(std::function<void()> on_written) = 0;

  private:
    std::unique_ptr<Writer> default_debug_writer_;
  };

  class Binding {
  public:
    virtual ~Binding();
    virtual std::vector<uint8_t> Get(const std::string& key) = 0;
    virtual void Set(const std::string& key, const uint8_t* data, int length) = 0;
  };

  explicit Game(std::unique_ptr<Driver> driver);
  Game(std::unique_ptr<Driver> driver, std::unique_ptr<Binding> binding);

  int Run();
  int Run(int argc, char *argv[]);
  int Run(const std::vector<std::string>& args);

private:
  void Update(Value f);

  std::unique_ptr<Driver> driver_;
  std::vector<Touch> touches_;
  std::vector<Gamepad> gamepads_;
  std::unique_ptr<Binding> binding_;
  bool is_audio_opened_ = false;
};

}

#endif  // {{.IncludeGuard}}
`))

var gameCppTmpl = template.Must(template.New("game.cpp").Parse(`// Code generated by go2cpp. DO NOT EDIT.

#include "{{.IncludePath}}game.h"

#include "{{.IncludePath}}gl.h"

#include <cstring>
#include <thread>

namespace {{.Namespace}} {

namespace {

// TODO: This is duplicated with js.go. Unify them?
void Panic(const std::string& msg) {
  std::cerr << msg << std::endl;
  __builtin_unreachable();
}

class DriverDebugWriter : public Writer {
public:
  DriverDebugWriter(Game::Driver* driver)
      : driver_{driver} {
  }

  void Write(const std::vector<uint8_t>& bytes) override {
    driver_->DebugWrite(bytes);
  }

private:
  Game::Driver* driver_;
};

class BindingObject : public Object {
public:
  explicit BindingObject(Game::Binding* binding)
      : binding_{binding} {
  }

  Value Get(const std::string& key) override {
    auto bytes = binding_->Get(key);
    auto u8 = std::make_shared<Uint8Array>(bytes.size());
    std::memcpy(u8->ToBytes().begin(), &(*bytes.begin()), bytes.size());
    return Value{u8};
  }

  void Set(const std::string& key, Value value) override {
    if (value.IsString()) {
      auto str = value.ToString();
      binding_->Set(key, reinterpret_cast<const uint8_t*>(&(*str.begin())), str.size());
      return;
    }
    if (value.IsObject()) {
      auto bytes = value.ToBytes();
      binding_->Set(key, &(*bytes.begin()), bytes.size());
      return;
    }
    Panic("BindingObject::Set: value must be a string or bytes but not: " + value.Inspect());
  }

  std::string ToString() const override {
    return "BindingObject";
  }

private:
  Game::Binding* binding_;
};

class LocalStorage : public Object {
public:
  explicit LocalStorage(Game::Driver* driver)
      : driver_{driver} {
  }

  Value Get(const std::string& key) override {
    if (key == "setItem") {
      return Value{std::make_shared<Function>(
        [this](Value self, std::vector<Value> args) -> Value {
          const std::string& key = args[0].ToString();
          const std::string& value = args[1].ToString();
          driver_->SetLocalStorageItem(key, value);
          return Value{};
        })};
    }
    if (key == "getItem") {
      return Value{std::make_shared<Function>(
        [this](Value self, std::vector<Value> args) -> Value {
          const std::string& key = args[0].ToString();
          const std::string& value = driver_->GetLocalStorageItem(key);
          return Value{value};
        })};
    }
    return Value{};
  }

  std::string ToString() const override {
    return "LocalStorage";
  }

private:
  Game::Driver* driver_;
};

class Navigator : public Object {
public:
  explicit Navigator(Game::Driver* driver)
      : driver_{driver} {
  }

  Value Get(const std::string& key) override {
    if (key == "language") {
      return Value{driver_->GetDefaultLanguage()};
    }
    if (key == "getGamepads") {
      return Value{std::make_shared<Function>(
        [this](Value self, std::vector<Value> args) -> Value {
          const std::vector<Game::Gamepad>& gamepads = driver_->GetGamepads();
          std::vector<Value> gamepad_values(gamepads.size());
          for (size_t i = 0; i < gamepads.size(); i++) {
            std::vector<Value> axes(gamepads[i].axis_count);
            for (size_t j = 0; j < axes.size(); j++) {
              axes[j] = Value{gamepads[i].axes[j]};
            }

            std::vector<Value> buttons(gamepads[i].button_count);
            for (size_t j = 0; j < buttons.size(); j++) {
              buttons[j] = Value{std::make_shared<DictionaryValues>(std::map<std::string, Value>{
                {"pressed", Value{gamepads[i].buttons[j]}},
              })};
            }

            gamepad_values[i] = Value{std::make_shared<DictionaryValues>(std::map<std::string, Value>{
              {"index", Value{static_cast<double>(gamepads[i].id)}},
              {"id", Value{"go2cpp gamepad " + std::to_string(gamepads[i].id)}},
              {"mapping", Value{gamepads[i].standard ? "standard" : ""}},
              {"axes", Value{axes}},
              {"buttons", Value{buttons}},
            })};
          }
          return Value{gamepad_values};
        })};
    }
    return Value{};
  }

  std::string ToString() const override {
    return "Navigator";
  }

private:
  Game::Driver* driver_;

};

class AudioPlayer : public Object {
public:
  explicit AudioPlayer(Game::Driver* driver)
      : driver_{driver} {
  }

  void SetOnWrittenCallback(Value on_written, std::function<void()> on_written_callback) {
    on_written_ = on_written;
    player_ = driver_->CreateAudioPlayer(on_written_callback);
  }

  void InvokeOnWrittenCallback() {
    if (closed_) {
      return;
    }
    on_written_.ToObject().Invoke({}, {});
  }

  Value Get(const std::string& key) override {
    if (key == "volume") {
      return Value{player_->GetVolume()};
    }
    if (key == "pause") {
      return Value{std::make_shared<Function>(
        [this](Value self, std::vector<Value> args) -> Value {
          player_->Pause();
          return Value{};
        })};
    }
    if (key == "play") {
      return Value{std::make_shared<Function>(
        [this](Value self, std::vector<Value> args) -> Value {
          player_->Play();
          return Value{};
        })};
    }
    if (key == "close") {
      return Value{std::make_shared<Function>(
        [this](Value self, std::vector<Value> args) -> Value {
          closed_ = true;
          bool immediately = args[0].ToBool();
          // Removing a player might cause joining its thread, which can take long.
          // Call Close explicitly.
          player_->Close(immediately);
          return Value{};
        })};
    }
    if (key == "write") {
      return Value{std::make_shared<Function>(
        [this](Value self, std::vector<Value> args) -> Value {
          BytesSpan buf = args[0].ToBytes();
          int size = static_cast<int>(args[1].ToNumber());
          player_->Write(buf.begin(), size);
          return Value{};
        })};
    }
    if (key == "unplayedBufferSize") {
      return Value{static_cast<double>(player_->GetUnplayedBufferSize())};
    }
    return Value{};
  }

  void Set(const std::string& key, Value value) override {
    if (key == "volume") {
      player_->SetVolume(value.ToNumber());
      return;
    }
  }

  std::string ToString() const override {
    return "AudioPlayer";
  }

private:
  Game::Driver* driver_;
  std::unique_ptr<Game::AudioPlayer> player_;
  Value buf_;
  Value on_written_;

  bool closed_ = false;
  std::mutex mutex_;
};

class Audio : public Object {
public:
  Audio(Go* go, Game::Driver* driver)
      : go_{go},
        driver_{driver} {
  }

  Value Get(const std::string& key) override {
    if (key == "createPlayer") {
      return Value{std::make_shared<Function>(
        [this](Value self, std::vector<Value> args) -> Value {
          auto p = std::make_shared<AudioPlayer>(driver_);
          // Capture the shared pointer and use it in the lambda.
          // The AudioPlayer must exist when invoking.
          p->SetOnWrittenCallback(args[0], [this, p]() {
            // This callback can be invoked from a different thread. Use EnqueueTask here.
            go_->EnqueueTask([p]() {
              p->InvokeOnWrittenCallback();
            });
          });
          return Value{p};
        })};
    }
    return Value{};
  }

  std::string ToString() const override {
    return "Audio";
  }

private:
  Go* go_;
  Game::Driver* driver_;
};

} // namespace

Game::AudioPlayer::~AudioPlayer() = default;

Game::Driver::~Driver() = default;

void Game::Driver::DebugWrite(const std::vector<uint8_t>& bytes) {
  if (!default_debug_writer_) {
    default_debug_writer_ = std::make_unique<StreamWriter>(std::cerr);
  }
  default_debug_writer_->Write(bytes);
}

std::string Game::Driver::GetDefaultLanguage() {
  return "en";
}

Game::Game(std::unique_ptr<Driver> driver)
  : Game(std::move(driver), nullptr) {
}

Game::Game(std::unique_ptr<Driver> driver, std::unique_ptr<Binding> binding)
  : driver_{std::move(driver)},
    binding_{std::move(binding)} {
}

int Game::Run() {
  return Run({});
}

int Game::Run(int argc, char *argv[]) {
  std::vector<std::string> args(argv, argv + argc);
  return Run(args);
}

int Game::Run(const std::vector<std::string>& args) {
  if (!driver_->Initialize()) {
    return EXIT_FAILURE;
  }

  auto& global = Value::Global().ToObject();
  global.Set("localStorage", Value{std::make_shared<LocalStorage>(driver_.get())});
  global.Set("navigator", Value{std::make_shared<Navigator>(driver_.get())});

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
  go2cpp->Set("getTouchId", Value{std::make_shared<Function>(
    [this](Value self, std::vector<Value> args) -> Value {
      int idx = static_cast<int>(args[0].ToNumber());
      return Value{static_cast<double>(touches_[idx].id)};
    })});
  go2cpp->Set("getTouchX", Value{std::make_shared<Function>(
    [this](Value self, std::vector<Value> args) -> Value {
      int idx = static_cast<int>(args[0].ToNumber());
      return Value{static_cast<double>(touches_[idx].x)};
    })});
  go2cpp->Set("getTouchY", Value{std::make_shared<Function>(
    [this](Value self, std::vector<Value> args) -> Value {
      int idx = static_cast<int>(args[0].ToNumber());
      return Value{static_cast<double>(touches_[idx].y)};
    })});

  Go go{std::make_unique<DriverDebugWriter>(driver_.get())};

  go2cpp->Set("createAudio", Value{std::make_shared<Function>(
    [this, &go](Value self, std::vector<Value> args) -> Value {
      int sample_rate = static_cast<int>(args[0].ToNumber());
      int channel_num = static_cast<int>(args[1].ToNumber());
      int bit_depth_in_bytes = static_cast<int>(args[2].ToNumber());

      driver_->OpenAudio(sample_rate, channel_num, bit_depth_in_bytes);
      is_audio_opened_ = true;
      return Value{std::make_shared<Audio>(&go, driver_.get())};
    })});

  if (binding_) {
    go2cpp->Set("binding", Value{std::make_shared<BindingObject>(binding_.get())});
  }

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

  int code = go.Run(args);
  if (is_audio_opened_) {
    driver_->CloseAudio();
  }
  if (!driver_->Finalize()) {
    if (code) {
      return code;
    }
    return EXIT_FAILURE;
  }
  return code;
}

void Game::Update(Value f) {
  auto& global = Value::Global().ToObject();
  auto& go2cpp = global.Get("go2cpp").ToObject();

  touches_ = driver_->GetTouches();
  go2cpp.Set("touchCount", Value{static_cast<double>(touches_.size())});

  f.ToObject().Invoke(Value{}, {});
}

Game::Binding::~Binding() = default;

}
`))
