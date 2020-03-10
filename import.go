// SPDX-License-Identifier: Apache-2.0

package main

var importFuncBodies = map[string]string{
	// func wasmExit(code int32)
	"runtime.wasmExit": `    var code = go.mem.LoadInt32(local0 + 8);
    go.exited = true;
    go.inst = null;
    go.values = null;
    go.goRefCounts = null;
    go.ids = null;
    go.idPool = null;
    go.Exit(code);`,

	// func wasmWrite(fd uintptr, p unsafe.Pointer, n int32)
	"runtime.wasmWrite": `    var fd = go.mem.LoadInt64(local0 + 8);
    if (fd != 1 && fd != 2)
    {
        throw new NotImplementedException($"fd for runtime.wasmWrite must be 1 or 2 but {fd}");
    }
    var p = go.mem.LoadInt64(local0 + 16);
    var n = go.mem.LoadInt32(local0 + 24);

    // Note that runtime.wasmWrite is used only for print/println so far.
    // Write the buffer to the standard output regardless of fd.
    go.DebugWrite(go.mem.LoadSliceDirectly(p, n));`,

	// func resetMemoryDataView()
	"runtime.resetMemoryDataView": `    // Do nothing.`,

	// func nanotime1() int64
	"runtime.nanotime1": `    go.mem.StoreInt64(local0 + 8, go.PreciseNowInNanoseconds());`,

	// func walltime1() (sec int64, nsec int32)
	"runtime.walltime1": `    var now = go.UnixNowInMilliseconds();
    go.mem.StoreInt64(local0 + 8, (long)(now / 1000));
    go.mem.StoreInt32(local0 + 16, (int)((now % 1000) * 1_000_000));`,

	// func scheduleTimeoutEvent(delay int64) int32
	"runtime.scheduleTimeoutEvent": `    var interval = go.mem.LoadInt64(local0 + 8);
    var id = go.SetTimeout((double)interval);
    go.mem.StoreInt32(local0 + 16, id);`,

	// func clearTimeoutEvent(id int32)
	"runtime.clearTimeoutEvent": `    var id = go.mem.LoadInt32(local0 + 8);
    go.ClearTimeout(id);`,

	// func getRandomData(r []byte)
	"runtime.getRandomData": `    var slice = go.mem.LoadSlice(local0 + 8);
    var bytes = go.GetRandomBytes(slice.Count);
    for (int i = 0; i < slice.Count; i++) {
        slice[i] = bytes[i];
    }`,

	// func finalizeRef(v ref)
	"syscall/js.finalizeRef": `    int id = (int)go.mem.LoadUint32(local0 + 8);
    go.goRefCounts[id]--;
    if (go.goRefCounts[id] == 0)
    {
        var v = go.values[id];
        go.values[id] = null;
        go.ids.Remove(v);
        go.idPool.Push(id);
    }`,

	// func stringVal(value string) ref
	"syscall/js.stringVal": `    go.StoreValue(local0 + 24, go.mem.LoadString(local0 + 8));`,

	// func valueGet(v ref, p string) ref
	"syscall/js.valueGet": `    var result = JSObject.ReflectGet(go.LoadValue(local0 + 8), go.mem.LoadString(local0 + 16));
    local0 = go.inst.getsp();
    go.StoreValue(local0 + 32, result);`,

	// func valueSet(v ref, p string, x ref)
	"syscall/js.valueSet": `    JSObject.ReflectSet(go.LoadValue(local0 + 8), go.mem.LoadString(local0 + 16), go.LoadValue(local0 + 32));`,

	// func valueDelete(v ref, p string)
	"syscall/js.valueDelete": `    JSObject.ReflectDelete(go.LoadValue(local0 + 8), go.mem.LoadString(local0 + 16));`,

	// func valueIndex(v ref, i int) ref
	"syscall/js.valueIndex": `    go.StoreValue(local0 + 24, JSObject.ReflectGet(go.LoadValue(local0 + 8), go.mem.LoadInt64(local0 + 16).ToString()));`,

	// valueSetIndex(v ref, i int, x ref)
	"syscall/js.valueSetIndex": `    JSObject.ReflectSet(go.LoadValue(local0 + 8), go.mem.LoadInt64(local0 + 16).ToString(), go.LoadValue(local0 + 24));`, /*

					// func valueCall(v ref, m string, args []ref) (ref, bool)
					"syscall/js.valueCall": (sp) => {
						try {
							const v = loadValue(sp + 8);
							const m = Reflect.get(v, loadString(sp + 16));
							const args = loadSliceOfValues(sp + 32);
							const result = Reflect.apply(m, v, args);
							sp = this._inst.exports.getsp(); // see comment above
							storeValue(sp + 56, result);
							this.mem.setUint8(sp + 64, 1);
						} catch (err) {
							storeValue(sp + 56, err);
							this.mem.setUint8(sp + 64, 0);
						}
					},*/

	// func valueInvoke(v ref, args []ref) (ref, bool)
	"syscall/js.valueInvoke": `    throw new NotImplementedException();`, /*(sp) => {
						try {
							const v = loadValue(sp + 8);
							const args = loadSliceOfValues(sp + 16);
							const result = Reflect.apply(v, undefined, args);
							sp = this._inst.exports.getsp(); // see comment above
							storeValue(sp + 40, result);
							this.mem.setUint8(sp + 48, 1);
						} catch (err) {
							storeValue(sp + 40, err);
							this.mem.setUint8(sp + 48, 0);
						}
					},*/

	// func valueNew(v ref, args []ref) (ref, bool)
	"syscall/js.valueNew": `    var v = go.LoadValue(local0 + 8);
    var args = go.LoadSliceOfValues(local0 + 16);
    var result = JSObject.ReflectConstruct(v, args);
    if (result != null)
    {
        local0 = go.inst.getsp();
        go.StoreValue(local0 + 40, result);
        go.mem.StoreInt8(local0 + 48, 1);
    }
    else
    {
        go.StoreValue(local0 + 40, null);
        go.mem.StoreInt8(local0 + 48, 0);
    }`, /*(sp) => {
						try {
							const v = loadValue(sp + 8);
							const args = loadSliceOfValues(sp + 16);
							const result = Reflect.construct(v, args);
							sp = this._inst.exports.getsp(); // see comment above
							storeValue(sp + 40, result);
							this.mem.setUint8(sp + 48, 1);
						} catch (err) {
							storeValue(sp + 40, err);
							this.mem.setUint8(sp + 48, 0);
						}
					},

					// func valueLength(v ref) int
					"syscall/js.valueLength": (sp) => {
						setInt64(sp + 16, parseInt(loadValue(sp + 8).length));
					},

					// valuePrepareString(v ref) (ref, int)
					"syscall/js.valuePrepareString": (sp) => {
						const str = encoder.encode(String(loadValue(sp + 8)));
						storeValue(sp + 16, str);
						setInt64(sp + 24, str.length);
					},

					// valueLoadString(v ref, b []byte)
					"syscall/js.valueLoadString": (sp) => {
						const str = loadValue(sp + 8);
						loadSlice(sp + 16).set(str);
					},

					// func valueInstanceOf(v ref, t ref) bool
					"syscall/js.valueInstanceOf": (sp) => {
						this.mem.setUint8(sp + 24, loadValue(sp + 8) instanceof loadValue(sp + 16));
					},

					// func copyBytesToGo(dst []byte, src ref) (int, bool)
					"syscall/js.copyBytesToGo": (sp) => {
						const dst = loadSlice(sp + 8);
						const src = loadValue(sp + 32);
						if (!(src instanceof Uint8Array)) {
							this.mem.setUint8(sp + 48, 0);
							return;
						}
						const toCopy = src.subarray(0, dst.length);
						dst.set(toCopy);
						setInt64(sp + 40, toCopy.length);
						this.mem.setUint8(sp + 48, 1);
					},*/

	// func copyBytesToJS(dst ref, src []byte) (int, bool)
	"syscall/js.copyBytesToJS": `    var dst = go.LoadValue(local0 + 8);
    var src = go.mem.LoadSlice(local0 + 16);
    if (!(dst is byte[]))
    {
        go.mem.StoreInt8(local0 + 48, 0);
        return;
    }
    var dstbs = (byte[])dst;
    for (int i = 0; i < dstbs.Length; i++)
    {
        dstbs[i] = src[i];
    }
    go.mem.StoreInt64(local0 + 40, (long)dstbs.Length);
    go.mem.StoreInt8(local0 + 48, 1);`,

	"debug": `    Console.WriteLine(local0);`,
}
