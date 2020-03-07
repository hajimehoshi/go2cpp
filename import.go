// SPDX-License-Identifier: Apache-2.0

package main

var importFuncBodies = map[string]string{
	// func wasmExit(code int32)
	"runtime.wasmExit": `    var code = go.mem.LoadInt32(local0 + 8);
    // TODO: go.exited = true;
    // TODO: go.inst = null; ?
    go.values = null;
    go.goRefCounts = null;
    go.ids = null;
    go.idPool = null;
    // TODO: Invoke exit function`,

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

	"debug": `    Console.WriteLine(local0);`,
}
