// SPDX-License-Identifier: Apache-2.0

package main

var importFuncBodies = map[string]string{
	/*"runtime.wasmExit": (sp) => {
		const code = this.mem.getInt32(sp + 8, true);
		this.exited = true;
		delete this._inst;
		delete this._values;
		delete this._goRefCounts;
		delete this._ids;
		delete this._idPool;
		this.exit(code);
	},

	// func wasmWrite(fd uintptr, p unsafe.Pointer, n int32)
	"runtime.wasmWrite": (sp) => {
		const fd = getInt64(sp + 8);
		const p = getInt64(sp + 16);
		const n = this.mem.getInt32(sp + 24, true);
		fs.writeSync(fd, new Uint8Array(this._inst.exports.mem.buffer, p, n));
	},

	// func resetMemoryDataView()
	"runtime.resetMemoryDataView": (sp) => {
		this.mem = new DataView(this._inst.exports.mem.buffer);
	},

	// func nanotime1() int64
	"runtime.nanotime1": (sp) => {
		setInt64(sp + 8, (timeOrigin + performance.now()) * 1000000);
	},

	// func walltime1() (sec int64, nsec int32)
	"runtime.walltime1": (sp) => {
		const msec = (new Date).getTime();
		setInt64(sp + 8, msec / 1000);
		this.mem.setInt32(sp + 16, (msec % 1000) * 1000000, true);
	},

	// func scheduleTimeoutEvent(delay int64) int32
	"runtime.scheduleTimeoutEvent": (sp) => {
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
	"runtime.getRandomData": `    var slice = mem.LoadSlice(local0 + 8);
    var bytes = new byte[slice.Count];
    rngCsp.GetBytes(bytes);
    for (int i = 0; i < slice.Count; i++) {
        slice[i] = bytes[i];
    }`,
	"debug": `    Console.WriteLine(local0);`,
}
