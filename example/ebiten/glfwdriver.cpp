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

void GLFWDriver::OpenAudio(int sample_rate, int channel_num,
                           int bit_depth_in_bytes, int buffer_size) {
  sample_rate_ = sample_rate;
  channel_num_ = channel_num;
  bit_depth_in_bytes_ = bit_depth_in_bytes;
  buffer_size_ = buffer_size;
}

int GLFWDriver::CreateAudioPlayer() {
  next_player_id_++;
  int player_id = next_player_id_;
  players_[player_id] = std::make_unique<AudioPlayer>(
      sample_rate_, channel_num_, bit_depth_in_bytes_, buffer_size_);
  return player_id;
}

double GLFWDriver::AudioPlayerGetVolume(int player_id) { return 1; }

void GLFWDriver::AudioPlayerSetVolume(int player_id, double volume) {}

void GLFWDriver::AudioPlayerPause(int player_id) {
  players_[player_id]->Pause();
}

void GLFWDriver::AudioPlayerPlay(int player_id) { players_[player_id]->Play(); }

void GLFWDriver::AudioPlayerClose(int player_id) {
  players_.erase(player_id);
}

void GLFWDriver::AudioPlayerWrite(int player_id, const uint8_t *data,
                                  int length) {
  players_[player_id]->Write(length);
}

bool GLFWDriver::AudioPlayerIsWritable(int player_id) {
  return players_[player_id]->IsWritable();
}

GLFWDriver::AudioPlayer::AudioPlayer(int sample_rate, int channel_num,
                                     int bit_depth_in_bytes, int buffer_size)
    : sample_rate_{sample_rate}, channel_num_{channel_num},
      bit_depth_in_bytes_{bit_depth_in_bytes}, buffer_size_{buffer_size},
      thread_{[this] {
        Loop();
      }} {}

GLFWDriver::AudioPlayer::~AudioPlayer() {
  Close();
  if (thread_.joinable()) {
    thread_.join();
  }
}

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

void GLFWDriver::AudioPlayer::Write(int length) {
  {
    std::unique_lock<std::mutex> lock{mutex_};
    cond_.wait(lock, [this] { return IsWritableImpl(); });
    if (closed_) {
      return;
    }
    written_ += length;
  }
  cond_.notify_one();
}

bool GLFWDriver::AudioPlayer::IsWritable() {
  std::lock_guard<std::mutex> lock{mutex_};
  return IsWritableImpl();
}

void GLFWDriver::AudioPlayer::Loop() {
  for (;;) {
    {
      std::unique_lock<std::mutex> lock{mutex_};
      cond_.wait(lock, [this] {
        return (written_ >= buffer_size_ || closed_) && !paused_;
      });
      if (closed_) {
        return;
      }
      written_ -= buffer_size_;
    }
    cond_.notify_one();
    int bytes_per_sec = sample_rate_ * channel_num_ * bit_depth_in_bytes_;
    std::chrono::duration<double> duration(
                                           static_cast<double>(buffer_size_) /
                                           static_cast<double>(bytes_per_sec));
    std::this_thread::sleep_for(duration);
  }
}

bool GLFWDriver::AudioPlayer::IsWritableImpl() const {
  return (written_ < buffer_size_ || closed_) && !paused_;
}

void GLFWDriver::AudioPlayer::Close() {
  {
    std::lock_guard<std::mutex> lock{mutex_};
    paused_ = false;
    closed_ = true;
  }
  cond_.notify_all();
}
