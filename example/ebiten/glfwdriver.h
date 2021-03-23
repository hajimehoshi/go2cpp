// SPDX-License-Identifier: Apache-2.0

#ifndef GLFWGAME_H
#define GLFWGAME_H

#include "autogen/game.h"

#include <condition_variable>
#include <functional>
#include <map>
#include <mutex>
#include <string>
#include <thread>
#include <vector>

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
  void OpenAudio(int sample_rate, int channel_num,
                 int bit_depth_in_bytes) override;
  std::string GetLocalStorageItem(const std::string &key) override;
  void SetLocalStorageItem(const std::string &key,
                           const std::string &value) override;
  std::unique_ptr<go2cpp_autogen::Game::AudioPlayer>
  CreateAudioPlayer(std::function<void()> on_written) override;

private:
  class AudioPlayer : public go2cpp_autogen::Game::AudioPlayer {
  public:
    AudioPlayer(int sample_rate, int channel_num, int bit_depth_in_bytes,
                int buffer_size, std::function<void()> on_written);
    ~AudioPlayer() override;

    void Close(bool immediately) override;
    double GetVolume() override;
    void SetVolume(double volume) override;
    void Pause() override;
    void Play() override;
    void Write(const uint8_t *data, int length) override;
    size_t GetUnplayedBufferSize() override;

  private:
    void Loop();

    const int sample_rate_;
    const int channel_num_;
    const int bit_depth_in_bytes_;
    const int buffer_size_;
    std::function<void()> on_written_;
    double volume_ = 1.0;
    int ready_to_play_ = 0;
    bool paused_ = false;
    bool closed_ = false;
    std::mutex mutex_;
    std::condition_variable cond_;
    std::thread thread_;
  };

  GLFWwindow *window_;
  double device_pixel_ratio_;
  std::map<std::string, std::string> local_storage_;
  int sample_rate_ = 0;
  int channel_num_ = 0;
  int bit_depth_in_bytes_ = 0;
  int buffer_size_ = 0;
};

#endif
