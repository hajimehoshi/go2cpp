// SPDX-License-Identifier: Apache-2.0

package gowasm2cpp

var importFuncBodies = map[string]string{
	// func wasmExit(code int32)
	"runtime.wasmExit": `  int32_t code = go_->mem_->LoadInt32(local0 + 8);
  go_->exited_ = true;
  go_->inst_.reset();
  go_->values_.clear();
  go_->go_ref_counts_.clear();
  go_->ids_.clear();
  go_->id_pool_ = std::stack<int32_t>();
  go_->Exit(code);`,

	// func wasmWrite(fd uintptr, p unsafe.Pointer, n int32)
	"runtime.wasmWrite": `  int64_t fd = go_->mem_->LoadInt64(local0 + 8);
  if (fd != 1 && fd != 2) {
    error("fd for runtime.wasmWrite must be 1 or 2 but " + std::to_string(fd));
  }
  int64_t p = go_->mem_->LoadInt64(local0 + 16);
  int32_t n = go_->mem_->LoadInt32(local0 + 24);

  // Note that runtime.wasmWrite is used only for print/println so far.
  // Write the buffer to the standard output regardless of fd.
  go_->DebugWrite(go_->mem_->LoadSliceDirectly(p, n));`,

	// func resetMemoryDataView()
	"runtime.resetMemoryDataView": `  // Do nothing.`,

	// func nanotime1() int64
	"runtime.nanotime1": `  go_->mem_->StoreInt64(local0 + 8, go_->PreciseNowInNanoseconds());`,

	// func walltime1() (sec int64, nsec int32)
	"runtime.walltime1": `  double now = go_->UnixNowInMilliseconds();
  go_->mem_->StoreInt64(local0 + 8, static_cast<int64_t>(now / 1000));
  go_->mem_->StoreInt32(local0 + 16, static_cast<int32_t>(std::fmod(now, 1000) * 1000000));`,

	// func scheduleTimeoutEvent(delay int64) int32
	"runtime.scheduleTimeoutEvent": `  int64_t interval = go_->mem_->LoadInt64(local0 + 8);
  int32_t id = go_->SetTimeout(static_cast<double>(interval));
  go_->mem_->StoreInt32(local0 + 16, id);`,

	// func clearTimeoutEvent(id int32)
	"runtime.clearTimeoutEvent": `  int32_t id = go_->mem_->LoadInt32(local0 + 8);
  go_->ClearTimeout(id);`,

	// func getRandomData(r []byte)
	"runtime.getRandomData": `  BytesSegment slice = go_->mem_->LoadSlice(local0 + 8);
  go_->GetRandomBytes(slice);`,

	// func finalizeRef(v ref)
	"syscall/js.finalizeRef": `  int32_t id = static_cast<int32_t>(go_->mem_->LoadUint32(local0 + 8));
  go_->go_ref_counts_[id]--;
  if (go_->go_ref_counts_[id] == 0) {
    Value v = go_->values_[id];
    go_->values_[id] = Value{};
    go_->ids_.erase(v);
    go_->id_pool_.push(id);
  }`,

	// func stringVal(value string) ref
	"syscall/js.stringVal": `  go_->StoreValue(local0 + 24, Value{go_->mem_->LoadString(local0 + 8)});`,

	// func valueGet(v ref, p string) ref
	"syscall/js.valueGet": `  Value result = JSObject::ReflectGet(go_->LoadValue(local0 + 8), go_->mem_->LoadString(local0 + 16));
  local0 = go_->inst_->getsp();
  go_->StoreValue(local0 + 32, result);`,

	// func valueSet(v ref, p string, x ref)
	"syscall/js.valueSet": `  JSObject::ReflectSet(go_->LoadValue(local0 + 8), go_->mem_->LoadString(local0 + 16), go_->LoadValue(local0 + 32));`,

	// func valueDelete(v ref, p string)
	"syscall/js.valueDelete": `  JSObject::ReflectDelete(go_->LoadValue(local0 + 8), go_->mem_->LoadString(local0 + 16));`,

	// func valueIndex(v ref, i int) ref
	"syscall/js.valueIndex": `  go_->StoreValue(local0 + 24, JSObject::ReflectGet(go_->LoadValue(local0 + 8), std::to_string(go_->mem_->LoadInt64(local0 + 16))));`,

	// valueSetIndex(v ref, i int, x ref)
	"syscall/js.valueSetIndex": `  JSObject::ReflectSet(go_->LoadValue(local0 + 8), std::to_string(go_->mem_->LoadInt64(local0 + 16)), go_->LoadValue(local0 + 24));`,

	// func valueCall(v ref, m string, args []ref) (ref, bool)
	"syscall/js.valueCall": `  Value v = go_->LoadValue(local0 + 8);
  Value m = JSObject::ReflectGet(v, go_->mem_->LoadString(local0 + 16));
  std::vector<Value> args = go_->LoadSliceOfValues(local0 + 32);
  Value result = JSObject::ReflectApply(m, v, args);
  local0 = go_->inst_->getsp();
  go_->StoreValue(local0 + 56, result);
  go_->mem_->StoreInt8(local0 + 64, 1);`,

	// func valueInvoke(v ref, args []ref) (ref, bool)
	"syscall/js.valueInvoke": `  Value v = go_->LoadValue(local0 + 8);
  std::vector<Value> args = go_->LoadSliceOfValues(local0 + 16);
  Value result = JSObject::ReflectApply(v, Value::Undefined(), args);
  local0 = go_->inst_->getsp();
  go_->StoreValue(local0 + 40, result);
  go_->mem_->StoreInt8(local0 + 48, 1);`,

	// func valueNew(v ref, args []ref) (ref, bool)
	"syscall/js.valueNew": `  Value v = go_->LoadValue(local0 + 8);
  std::vector<Value> args = go_->LoadSliceOfValues(local0 + 16);
  Value result = JSObject::ReflectConstruct(v, args);
  if (!result.IsNull()) {
    local0 = go_->inst_->getsp();
    go_->StoreValue(local0 + 40, result);
    go_->mem_->StoreInt8(local0 + 48, 1);
  } else {
    go_->StoreValue(local0 + 40, Value{});
    go_->mem_->StoreInt8(local0 + 48, 0);
  }`,

	// func valueLength(v ref) int
	"syscall/js.valueLength": `  go_->mem_->StoreInt64(local0 + 16, static_cast<int64_t>(go_->LoadValue(local0 + 8).ToArray().size()));`,

	// valuePrepareString(v ref) (ref, int)
	"syscall/js.valuePrepareString": `  std::string str = go_->LoadValue(local0 + 8).ToString();
  go_->StoreValue(local0 + 16, Value{str});
  go_->mem_->StoreInt64(local0 + 24, static_cast<int64_t>(str.size()));`,

	// valueLoadString(v ref, b []byte)
	"syscall/js.valueLoadString": `  std::string src = go_->LoadValue(local0 + 8).ToString();
  BytesSegment dst = go_->mem_->LoadSlice(local0 + 16);
  int len = std::min(dst.size(), src.size());
  std::copy(src.begin(), src.begin() + len, dst.begin());`,

	/*// func valueInstanceOf(v ref, t ref) bool
	"syscall/js.valueInstanceOf": (sp) => {
		this.mem_->setUint8(sp + 24, loadValue(sp + 8) instanceof loadValue(sp + 16));
	},*/

	// func copyBytesToGo(dst []byte, src ref) (int, bool)
	"syscall/js.copyBytesToGo": `  BytesSegment dst = go_->mem_->LoadSlice(local0 + 8);
  Value src = go_->LoadValue(local0 + 32);
  if (!src.IsBytes()) {
    go_->mem_->StoreInt8(local0 + 48, 0);
    return;
  }
  std::vector<uint8_t>& srcbs = src.ToBytes();
  std::copy(srcbs.begin(), srcbs.end(), dst.begin());
  go_->mem_->StoreInt64(local0 + 40, static_cast<int64_t>(dst.size()));
  go_->mem_->StoreInt8(local0 + 48, 1);`,

	// func copyBytesToJS(dst ref, src []byte) (int, bool)
	"syscall/js.copyBytesToJS": `  Value dst = go_->LoadValue(local0 + 8);
  BytesSegment src = go_->mem_->LoadSlice(local0 + 16);
  if (!dst.IsBytes()) {
    go_->mem_->StoreInt8(local0 + 48, 0);
    return;
  }
  std::vector<uint8_t>& dstbs = dst.ToBytes();
  std::copy(src.begin(), src.end(), dstbs.begin());
  go_->mem_->StoreInt64(local0 + 40, static_cast<int64_t>(dstbs.size()));
  go_->mem_->StoreInt8(local0 + 48, 1);`,

	"debug": `  std::cout << local0 << std::endl;`,
}
