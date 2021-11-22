// SPDX-License-Identifier: Apache-2.0

package gowasm2cpp

var importFuncBodies = map[string]string{
	// func wasmExit(code int32)
	"runtime.wasmExit": `  int32_t code = go_->mem_->LoadInt32(local0_ + 8);
  go_->exited_ = true;
  // wasm_exec.js resets the members here, but do not reset members here.
  // Resetting them causes use-after-free. This can be detected by the address sanitizer.
  go_->Exit(code);`,

	// func wasmWrite(fd uintptr, p unsafe.Pointer, n int32)
	"runtime.wasmWrite": `  int64_t fd = go_->mem_->LoadInt64(local0_ + 8);
  if (fd != 1 && fd != 2) {
    error("fd for runtime.wasmWrite must be 1 or 2 but " + std::to_string(fd));
  }
  int64_t p = go_->mem_->LoadInt64(local0_ + 16);
  int32_t n = go_->mem_->LoadInt32(local0_ + 24);

  // Note that runtime.wasmWrite is used only for print/println so far.
  // Write the buffer to the standard error regardless of fd.
  go_->DebugWrite(go_->mem_->LoadSliceDirectly(p, n));`,

	// func resetMemoryDataView()
	"runtime.resetMemoryDataView": `  // Do nothing.`,

	// func nanotime1() int64
	"runtime.nanotime1": `  go_->mem_->StoreInt64(local0_ + 8, go_->PreciseNowInNanoseconds());`,

	// func walltime() (sec int64, nsec int32)
	"runtime.walltime": `  double now = go_->UnixNowInMilliseconds();
  go_->mem_->StoreInt64(local0_ + 8, static_cast<int64_t>(now / 1000));
  go_->mem_->StoreInt32(local0_ + 16, static_cast<int32_t>(std::fmod(now, 1000) * 1000000));`,

	// func scheduleTimeoutEvent(delay int64) int32
	"runtime.scheduleTimeoutEvent": `  int64_t interval = go_->mem_->LoadInt64(local0_ + 8);
  int32_t id = go_->SetTimeout(static_cast<double>(interval));
  go_->mem_->StoreInt32(local0_ + 16, id);`,

	// func clearTimeoutEvent(id int32)
	"runtime.clearTimeoutEvent": `  int32_t id = go_->mem_->LoadInt32(local0_ + 8);
  go_->ClearTimeout(id);`,

	// func getRandomData(r []byte)
	"runtime.getRandomData": `  BytesSpan slice = go_->mem_->LoadSlice(local0_ + 8);
  go_->GetRandomBytes(slice);`,

	// func finalizeRef(v ref)
	"syscall/js.finalizeRef": `  int32_t id = static_cast<int32_t>(go_->mem_->LoadUint32(local0_ + 8));
  go_->go_ref_counts_[id]--;
  if (go_->go_ref_counts_[id] == 0) {
    go_->finalizing_ids_.insert(id);
  }`,

	// func stringVal(value string) ref
	"syscall/js.stringVal": `  go_->StoreValue(local0_ + 24, Value{go_->mem_->LoadString(local0_ + 8)});`,

	// func valueGet(v ref, p string) ref
	"syscall/js.valueGet": `  Value result = Value::ReflectGet(go_->LoadValue(local0_ + 8), go_->mem_->LoadString(local0_ + 16));
  local0_ = go_->inst_->getsp();
  go_->StoreValue(local0_ + 32, result);`,

	// func valueSet(v ref, p string, x ref)
	"syscall/js.valueSet": `  Value::ReflectSet(go_->LoadValue(local0_ + 8), go_->mem_->LoadString(local0_ + 16), go_->LoadValue(local0_ + 32));`,

	// func valueDelete(v ref, p string)
	"syscall/js.valueDelete": `  Value::ReflectDelete(go_->LoadValue(local0_ + 8), go_->mem_->LoadString(local0_ + 16));`,

	// func valueIndex(v ref, i int) ref
	"syscall/js.valueIndex": `  go_->StoreValue(local0_ + 24, Value::ReflectGet(go_->LoadValue(local0_ + 8), std::to_string(go_->mem_->LoadInt64(local0_ + 16))));`,

	// valueSetIndex(v ref, i int, x ref)
	"syscall/js.valueSetIndex": `  Value::ReflectSet(go_->LoadValue(local0_ + 8), std::to_string(go_->mem_->LoadInt64(local0_ + 16)), go_->LoadValue(local0_ + 24));`,

	// func valueCall(v ref, m string, args []ref) (ref, bool)
	"syscall/js.valueCall": `  Value v = go_->LoadValue(local0_ + 8);
  Value m = Value::ReflectGet(v, go_->mem_->LoadString(local0_ + 16));
  std::vector<Value> args = go_->LoadSliceOfValues(local0_ + 32);
  Value result = Value::ReflectApply(m, v, args);
  local0_ = go_->inst_->getsp();
  go_->StoreValue(local0_ + 56, result);
  go_->mem_->StoreInt8(local0_ + 64, 1);`,

	// func valueInvoke(v ref, args []ref) (ref, bool)
	"syscall/js.valueInvoke": `  Value v = go_->LoadValue(local0_ + 8);
  std::vector<Value> args = go_->LoadSliceOfValues(local0_ + 16);
  Value result = Value::ReflectApply(v, Value{}, args);
  local0_ = go_->inst_->getsp();
  go_->StoreValue(local0_ + 40, result);
  go_->mem_->StoreInt8(local0_ + 48, 1);`,

	// func valueNew(v ref, args []ref) (ref, bool)
	"syscall/js.valueNew": `  Value v = go_->LoadValue(local0_ + 8);
  std::vector<Value> args = go_->LoadSliceOfValues(local0_ + 16);
  Value result = Value::ReflectConstruct(v, args);
  if (!result.IsUndefined()) {
    local0_ = go_->inst_->getsp();
    go_->StoreValue(local0_ + 40, result);
    go_->mem_->StoreInt8(local0_ + 48, 1);
  } else {
    go_->StoreValue(local0_ + 40, Value{});
    go_->mem_->StoreInt8(local0_ + 48, 0);
  }`,

	// func valueLength(v ref) int
	"syscall/js.valueLength": `  go_->mem_->StoreInt64(local0_ + 16, static_cast<int64_t>(go_->LoadValue(local0_ + 8).ToArray().size()));`,

	// valuePrepareString(v ref) (ref, int)
	"syscall/js.valuePrepareString": `  std::string str = go_->LoadValue(local0_ + 8).ToString();
  go_->StoreValue(local0_ + 16, Value{str});
  go_->mem_->StoreInt64(local0_ + 24, static_cast<int64_t>(str.size()));`,

	// valueLoadString(v ref, b []byte)
	"syscall/js.valueLoadString": `  std::string src = go_->LoadValue(local0_ + 8).ToString();
  BytesSpan dst = go_->mem_->LoadSlice(local0_ + 16);
  int len = std::min(dst.size(), src.size());
  std::memcpy(dst.begin(), &(*src.begin()), len);`,

	/*// func valueInstanceOf(v ref, t ref) bool
	"syscall/js.valueInstanceOf": (sp) => {
		this.mem_->setUint8(sp + 24, loadValue(sp + 8) instanceof loadValue(sp + 16));
	},*/

	// func copyBytesToGo(dst []byte, src ref) (int, bool)
	"syscall/js.copyBytesToGo": `  BytesSpan dst = go_->mem_->LoadSlice(local0_ + 8);
  Value src = go_->LoadValue(local0_ + 32);
  if (!src.IsBytes()) {
    go_->mem_->StoreInt8(local0_ + 48, 0);
    return;
  }
  BytesSpan srcbs = src.ToBytes();
  std::memcpy(dst.begin(), srcbs.begin(), std::min(srcbs.size(), dst.size()));
  go_->mem_->StoreInt64(local0_ + 40, static_cast<int64_t>(dst.size()));
  go_->mem_->StoreInt8(local0_ + 48, 1);`,

	// func copyBytesToJS(dst ref, src []byte) (int, bool)
	"syscall/js.copyBytesToJS": `  Value dst = go_->LoadValue(local0_ + 8);
  BytesSpan src = go_->mem_->LoadSlice(local0_ + 16);
  if (!dst.IsBytes()) {
    go_->mem_->StoreInt8(local0_ + 48, 0);
    return;
  }
  BytesSpan dstbs = dst.ToBytes();
  std::memcpy(dstbs.begin(), src.begin(), std::min(src.size(), dstbs.size()));
  go_->mem_->StoreInt64(local0_ + 40, static_cast<int64_t>(dstbs.size()));
  go_->mem_->StoreInt8(local0_ + 48, 1);`,

	"debug": `  std::cout << local0_ << std::endl;`,
}

func init() {
	// Add an old name for backward compatibility with Go 1.16 and before.
	importFuncBodies["runtime.walltime1"] = importFuncBodies["runtime.walltime"]
}
