// SPDX-License-Identifier: Apache-2.0

package main

const js = `    public delegate object JSFunc(object self, object[] args);

    sealed class JSObject
    {
        private const string defaultName = "(JSObject)";

        public static JSObject Undefined = new JSObject("undefined", null);
        public static JSObject Global;

        static JSObject()
        {
            JSObject obj = new JSObject("Object", null);
            JSObject arr = new JSObject("Array", null);
            JSObject process = new JSObject("process", null);
            JSObject fs = new JSObject("fs", new Dictionary<string, object>()
            {
                {"constants", new JSObject(defaultName, new Dictionary<string, object>()
                    {
                        {"O_WRONLY", -1},
                        {"O_RDWR", -1},
                        {"O_CREAT", -1},
                        {"O_TRUNC", -1},
                        {"O_APPEND", -1},
                        {"O_EXCL", -1},
                    })},
            });
            JSObject u8 = new JSObject("Uint8Array", null, (object self, object[] args) =>
            {
                if (args.Length == 0)
                {
                    return new byte[0];
                }
                if (args.Length == 1)
                {
                    var len = args[0];
                    if (len is double)
                    {
                        return new byte[(int)(double)len];
                    }
                    throw new NotImplementedException($"new Uint8Array({args[0]}) is not implemented");
                }
                throw new NotImplementedException($"new Uint8Array with {args.Length} args is not implemented");
            }, true);

            Global = new JSObject("global", new Dictionary<string, object>()
            {
                {"Object", obj},
                {"Array", arr},
                {"process", process},
                {"fs", fs},
                {"Uint8Array", u8},
            });
        }
        
        public static object ReflectGet(object target, string key)
        {
            if (target == Undefined)
            {
                throw new Exception($"get on undefined (key: {key}) is forbidden");
            }
            if (target == null)
            {
                throw new Exception($"get on null (key: {key}) is forbidden");
            }
            if (target is JSObject)
            {
                return ((JSObject)target).Get(key);
            }
            throw new Exception($"{target}.{key} not found");
        }

        public static void ReflectSet(object target, string key, object value)
        {
            if (target == Undefined)
            {
                throw new Exception($"set on undefined (key: {key}) is forbidden");
            }
            if (target == null)
            {
                throw new Exception($"set on null (key: {key}) is forbidden");
            }
            if (target is JSObject)
            {
                ((JSObject)target).Set(key, value);
                return;
            }
            throw new Exception($"{target}.{key} cannot be set");
        }

        public static void ReflectDelete(object target, string key)
        {
            if (target == Undefined)
            {
                throw new Exception($"delete on undefined is forbidden");
            }
            if (target == null)
            {
                throw new Exception($"delete on null is forbidden");
            }
            if (target is JSObject)
            {
                ((JSObject)target).Delete(key);
                return;
            }
            throw new Exception($"{target}.{key} cannot be deleted");
        }

        public static object ReflectConstruct(object target, object[] args)
        {
            if (target == Undefined)
            {
                throw new Exception($"new on undefined is forbidden");
            }
            if (target == null)
            {
                throw new Exception($"new on null is forbidden");
            }
            if (target is JSObject)
            {
                var t = (JSObject)target;
                if (t.ctor)
                {
                    return t.fn(t, args);
                }
                else
                {
                    throw new Exception($"{t} is not a constructor");
                }
            }
            throw new NotImplementedException($"new {target}({args}) cannot be called");
        }

        public static object ReflectApply(object target, object self, object[] args)
        {
            if (target == Undefined)
            {
                throw new Exception($"apply on undefined is forbidden");
            }
            if (target == null)
            {
                throw new Exception($"apply on null is forbidden");
            }
            if (target is JSObject)
            {
                var t = (JSObject)target;
                if (!t.ctor)
                {
                    return t.fn(self, args);
                }
                else
                {
                    throw new Exception($"{t} is a constructor");
                }
            }
            throw new NotImplementedException($"new {target}({args}) cannot be called");
        }

        private JSObject(string name, Dictionary<string, object> values)
            : this(name, values, null, false)
        {
        }

        private JSObject(string name, Dictionary<string, object> values, JSFunc fn, bool ctor)
        {
            this.name = name;
            this.values = values;
            this.fn = fn;
            this.ctor = ctor;
        }

        public bool IsFunction
        {
            get { return this.fn != null; }
        }

        public object Get(string key)
        {
            if (this.values != null && this.values.ContainsKey(key))
            {
                return this.values[key];
            }
            throw new Exception($"{this}.{key} not found");
        }

        public void Set(string key, object value)
        {
            if (this.values == null)
            {
                this.values = new Dictionary<string, object>();
            }
            this.values[key] = value;
        }

        public void Delete(string key)
        {
            if (this.values == null)
            {
                return;
            }
            this.values.Remove(key);
        }

        public override string ToString()
        {
            return this.name;
        }

        private Dictionary<string, object> values;
        private string name;
        private JSFunc fn;
        private bool ctor = false;
    }`
