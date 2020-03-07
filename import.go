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
        throw new NotImplementedException(string.Format("fd for runtime.wasmWrite must be 1 or 2 but {0}", fd));
    }
    var p = go.mem.LoadInt64(local0 + 16);
    var n = go.mem.LoadInt32(local0 + 24);

    // Note that runtime.wasmWrite is used only for print/println so far.
    // Write the buffer to the standard output regardless of fd.
    go.DebugWrite(go.mem.LoadSliceDirectly(p, n));`,

	// func resetMemoryDataView()
	"runtime.resetMemoryDataView": `    // Do nothing.`,

	// func nanotime1() int64
	"runtime.nanotime1": `    go.mem.StoreInt64(local0 + 8, go.Now());`,

	/*// func walltime1() (sec int64, nsec int32)
	"runtime.walltime1": (sp) => {
		const msec = (new Date).getTime();
		setInt64(sp + 8, msec / 1000);
		this.mem.setInt32(sp + 16, (msec % 1000) * 1000000, true);
	},*/

	// func scheduleTimeoutEvent(delay int64) int32
	/*"runtime.scheduleTimeoutEvent": (sp) => {
		const id = this._nextCallbackTimeoutID;
		this._nextCallbackTimeoutID++;
		this._scheduledTimeouts.set(id, setTimeout(
			() => {
				this._resume();
				while (this._scheduledTimeouts.has(id)) {
					// for some reason Go failed to register the timeout event, log and try again
					// (temporary workaround for https://github.com/golang/go/issues/28975)
					console.warn("scheduleTimeoutEvent: missed timeout event");
					this._resume();
				}
			},
			getInt64(sp + 8) + 1, // setTimeout has been seen to fire up to 1 millisecond early
		));
		this.mem.setInt32(sp + 16, id, true);
	},

	// func clearTimeoutEvent(id int32)
	"runtime.clearTimeoutEvent": (sp) => {
		const id = this.mem.getInt32(sp + 8, true);
		clearTimeout(this._scheduledTimeouts.get(id));
		this._scheduledTimeouts.delete(id);
	},*/

	// func getRandomData(r []byte)
	"runtime.getRandomData": `    var slice = go.mem.LoadSlice(local0 + 8);
    var bytes = go.GetRandomBytes(slice.Count);
    for (int i = 0; i < slice.Count; i++) {
        slice[i] = bytes[i];
    }`,
	"debug": `    Console.WriteLine(local0);`,
}
