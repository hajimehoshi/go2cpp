// SPDX-License-Identifier: Apache-2.0

#include "glfwdriver.h"

#include <GLFW/glfw3.h>

#include <chrono>
#include <dlfcn.h>
#include <thread>

namespace {

constexpr int kWidth = 640;
constexpr int kHeight = 480;

} // namespace

bool GLFWDriver::Init() {
  if (!glfwInit()) {
    return false;
  }

  glfwWindowHint(GLFW_CLIENT_API, GLFW_OPENGL_API);
  glfwWindowHint(GLFW_CONTEXT_VERSION_MAJOR, 2);
  glfwWindowHint(GLFW_CONTEXT_VERSION_MINOR, 1);

  // TODO: Close the window at the destructor.
  window_ = glfwCreateWindow(kWidth, kHeight, "Ebiten test", nullptr, nullptr);
  if (!window_) {
    glfwTerminate();
    return false;
  }
  glfwMakeContextCurrent(window_);
  glfwSwapInterval(1);

  int framebuffer_width;
  glfwGetFramebufferSize(window_, &framebuffer_width, nullptr);
  device_pixel_ratio_ = static_cast<double>(framebuffer_width) / kWidth;

  return true;
}

void GLFWDriver::Update(std::function<void()> f) {
  glfwPollEvents();
  f();
  glfwSwapBuffers(window_);
}

int GLFWDriver::GetScreenWidth() { return kWidth; }

int GLFWDriver::GetScreenHeight() { return kHeight; }

double GLFWDriver::GetDevicePixelRatio() { return device_pixel_ratio_; }

void *GLFWDriver::GetOpenGLFunction(const char *name) {
  return dlsym(RTLD_DEFAULT, name);
}

std::vector<go2cpp_autogen::Game::Touch> GLFWDriver::GetTouches() {
  if (glfwGetMouseButton(window_, GLFW_MOUSE_BUTTON_LEFT) != GLFW_PRESS) {
    return {};
  }

  double xpos, ypos;
  glfwGetCursorPos(window_, &xpos, &ypos);
  go2cpp_autogen::Game::Touch touch;
  touch.id = 0;
  touch.x = static_cast<int>(xpos);
  touch.y = static_cast<int>(ypos);
  return {touch};
}

std::vector<go2cpp_autogen::Game::Gamepad> GLFWDriver::GetGamepads() {
  std::vector<go2cpp_autogen::Game::Gamepad> gamepads;
  for (int id = GLFW_JOYSTICK_1; id <= GLFW_JOYSTICK_LAST; id++) {
    if (!glfwJoystickPresent(id)) {
      continue;
    }

    go2cpp_autogen::Game::Gamepad gamepad;
    gamepad.id = id;

    const unsigned char *button_states =
        glfwGetJoystickButtons(id, &gamepad.button_count);
    constexpr int kButtonMaxCount =
        sizeof(gamepad.buttons) / sizeof(gamepad.buttons[0]);
    if (kButtonMaxCount < gamepad.button_count) {
      gamepad.button_count = kButtonMaxCount;
    }
    for (int i = 0; i < gamepad.button_count; i++) {
      gamepad.buttons[i] = button_states[i] == GLFW_PRESS;
    }

    const float *axis_states = glfwGetJoystickAxes(id, &gamepad.axis_count);
    constexpr int kAxisMaxCount =
        sizeof(gamepad.axes) / sizeof(gamepad.axes[0]);
    if (kAxisMaxCount < gamepad.axis_count) {
      gamepad.axis_count = kAxisMaxCount;
    }
    for (int i = 0; i < gamepad.axis_count; i++) {
      gamepad.axes[i] = axis_states[i];
    }

    gamepads.push_back(gamepad);
  }
  return gamepads;
}

std::string GLFWDriver::GetLocalStorageItem(const std::string &key) {
  return local_storage_[key];
}

void GLFWDriver::SetLocalStorageItem(const std::string &key,
                                     const std::string &value) {
  local_storage_[key] = value;
}

void GLFWDriver::OpenAudio(int sample_rate, int channel_num,
                           int bit_depth_in_bytes) {
  sample_rate_ = sample_rate;
  channel_num_ = channel_num;
  bit_depth_in_bytes_ = bit_depth_in_bytes;
  buffer_size_ = sample_rate * channel_num * bit_depth_in_bytes / 2;
}

std::unique_ptr<go2cpp_autogen::Game::AudioPlayer>
GLFWDriver::CreateAudioPlayer(std::function<void()> on_written) {
  return std::make_unique<AudioPlayer>(sample_rate_, channel_num_,
                                       bit_depth_in_bytes_, buffer_size_,
                                       on_written);
}

GLFWDriver::AudioPlayer::AudioPlayer(int sample_rate, int channel_num,
                                     int bit_depth_in_bytes, int buffer_size,
                                     std::function<void()> on_written)
    : sample_rate_{sample_rate}, channel_num_{channel_num},
      bit_depth_in_bytes_{bit_depth_in_bytes}, buffer_size_{buffer_size},
      on_written_{on_written}, thread_{[this] { Loop(); }} {}

GLFWDriver::AudioPlayer::~AudioPlayer() {
  if (thread_.joinable()) {
    thread_.join();
  }
}

void GLFWDriver::AudioPlayer::Close(bool immediately) {
  {
    std::lock_guard<std::mutex> lock{mutex_};
    paused_ = false;
    closed_ = true;
  }
  cond_.notify_all();
}

double GLFWDriver::AudioPlayer::GetVolume() { return volume_; }

void GLFWDriver::AudioPlayer::SetVolume(double volume) { volume_ = volume; }

void GLFWDriver::AudioPlayer::Pause() {
  {
    std::lock_guard<std::mutex> lock{mutex_};
    if (closed_) {
      return;
    }
    paused_ = true;
  }
  cond_.notify_all();
}

void GLFWDriver::AudioPlayer::Play() {
  {
    std::lock_guard<std::mutex> lock{mutex_};
    if (closed_) {
      return;
    }
    paused_ = false;
  }
  cond_.notify_all();
}

void GLFWDriver::AudioPlayer::Write(const uint8_t *data, int length) {
  {
    std::unique_lock<std::mutex> lock{mutex_};
    cond_.wait(lock, [this] { return IsWritableImpl(); });
    if (closed_) {
      return;
    }
    ready_to_play_ += length;
  }
  cond_.notify_one();
}

bool GLFWDriver::AudioPlayer::IsWritable() {
  std::lock_guard<std::mutex> lock{mutex_};
  return IsWritableImpl();
}

size_t GLFWDriver::AudioPlayer::GetUnplayedBufferSize() {
  std::lock_guard<std::mutex> lock{mutex_};
  return ready_to_play_;
}

void GLFWDriver::AudioPlayer::Loop() {
  for (;;) {
    {
      std::unique_lock<std::mutex> lock{mutex_};
      cond_.wait(lock, [this] {
        return (ready_to_play_ >= buffer_size_ || closed_) && !paused_;
      });
      if (closed_) {
        return;
      }
      ready_to_play_ -= buffer_size_;
      on_written_();
    }
    cond_.notify_one();
    int bytes_per_sec = sample_rate_ * channel_num_ * bit_depth_in_bytes_;
    std::chrono::duration<double> duration(static_cast<double>(buffer_size_) /
                                           static_cast<double>(bytes_per_sec));
    std::this_thread::sleep_for(duration);
  }
}

bool GLFWDriver::AudioPlayer::IsWritableImpl() const {
  return (ready_to_play_ < buffer_size_ || closed_) && !paused_;
}
