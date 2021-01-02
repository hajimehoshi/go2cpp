// SPDX-License-Identifier: Apache-2.0

#ifndef GLFWGAME_H
#define GLFWGAME_H

#include "autogen/game.h"

struct GLFWwindow;

class GLFWDriver : public go2cpp_autogen::Game::Driver {
public:
  bool Init() override;
  void Update(std::function<void()> f) override;
  int GetScreenWidth() override;
  int GetScreenHeight() override;
  double GetDevicePixelRatio() override;
  void *GetOpenGLFunction(const char *name) override;
  std::vector<go2cpp_autogen::Game::Touch> GetTouches() override;
  std::vector<go2cpp_autogen::Game::Gamepad> GetGamepads() override;
  void OpenAudio(int sample_rate_, int channel_num_, int bit_depth_in_bytes_,
                 int buffer_size) override;
  int SendDataToAudio(const uint8_t *data, int length) override;

private:
  GLFWwindow *window_;
  double device_pixel_ratio_;
  int sample_rate_ = 0;
  int channel_num_ = 0;
  int bit_depth_in_bytes_ = 0;
};

#endif
