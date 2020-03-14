// SPDX-License-Identifier: Apache-2.0

package main

const js = `    public delegate object JSFunc(object self, object[] args);

    sealed class Writer
    {
        public Writer(TextWriter writer)
        {
            this.writer = writer;
        }

        public void Write(IEnumerable<byte> bytes)
        {
            this.buf.AddRange(bytes);
            while (this.buf.Contains((byte)'\n'))
            {
                var idx = this.buf.IndexOf((byte)'\n');
                var str = Encoding.UTF8.GetString(this.buf.GetRange(0, idx).ToArray());
                this.writer.WriteLine(str);
                this.buf.RemoveRange(0, idx+1);
            }
        }

        private TextWriter writer;
        private List<byte> buf = new List<byte>();
    }

    public interface IInvokable
    {
        object Invoke(object[] args);
    }

    sealed class JSObject : IInvokable
    {
        public interface IValues
        {
            object Get(string key);
            void Set(string key, object value);
            void Remove(string key);
        }

        private class DictionaryValues : IValues
        {
            public DictionaryValues(Dictionary<string, object> dict)
            {
                this.dict = dict;
            }

            public object Get(string key)
            {
                if (!this.dict.ContainsKey(key))
                {
                    throw new KeyNotFoundException(key);
                }
                return this.dict[key];
            }

            public void Set(string key, object value)
            {
                this.dict[key] = value;
            }

            public void Remove(string key)
            {
                this.dict.Remove(key);
            }

            private Dictionary<string, object> dict;
        }

        private class DotNetRootValues : IValues
        {
            public object Get(string key)
            {
                Type type = Type.GetType(key);
                if (type == null)
                {
                    return null;
                }
                return new JSObject(key, new DotNetTypeValues(type), (object self, object[] args) =>
                {
                    BindingFlags flags = BindingFlags.Public | BindingFlags.NonPublic | BindingFlags.Instance;
                    object inst = Activator.CreateInstance(type, flags, null, args, null, null);
                    return new JSObject(key, new DotNetInstanceValues(inst));
                }, true);
            }

            public void Set(string key, object value)
            {
                throw new Exception($"setting ${key} on a DotNetRootValue is forbidden");
            }

            public void Remove(string key)
            {
                throw new Exception($"removing ${key} on a DotNetRootValue is forbidden");
            }
        }

        private class DotNetTypeValues : IValues
        {
            public DotNetTypeValues(Type type)
            {
                this.type = type;
            }

            public object Get(string key)
            {
                BindingFlags flags = BindingFlags.Public | BindingFlags.NonPublic | BindingFlags.Static;
                FieldInfo field = this.type.GetField(key, flags);
                if (field != null)
                {
                    return field.GetValue(null);
                }
                PropertyInfo prop = this.type.GetProperty(key, flags);
                if (prop != null)
                {
                    return prop.GetValue(null);
                }
                MethodInfo method = this.type.GetMethod(key, flags);
                if (method != null)
                {
                    return new JSObject(key, null, (object self, object[] args) => {
                        return method.Invoke(null, args);
                    }, false);
                }
                return null;
            }

            public void Set(string key, object value)
            {
                BindingFlags flags = BindingFlags.Public | BindingFlags.NonPublic | BindingFlags.Static;
                FieldInfo field = this.type.GetField(key, flags);
                if (field != null)
                {
                    field.SetValue(null, value);
                    return;
                }
                PropertyInfo prop = this.type.GetProperty(key, flags);
                if (prop != null)
                {
                    prop.SetValue(null, value);
                    return;
                }
                throw new Exception($"setting {key} on {type} is forbidden");
            }

            public void Remove(string key)
            {
                throw new Exception($"removing ${key} on a DotNetTypeValue is forbidden");
            }

            private Type type;
        }

        private class DotNetInstanceValues : IValues
        {
            public DotNetInstanceValues(object obj)
            {
                this.obj = obj;
            }

            public object Get(string key)
            {
                BindingFlags flags = BindingFlags.Public | BindingFlags.NonPublic | BindingFlags.Instance;
                FieldInfo field = this.obj.GetType().GetField(key, flags);
                if (field != null)
                {
                    return field.GetValue(this.obj);
                }
                PropertyInfo prop = this.obj.GetType().GetProperty(key, flags);
                if (prop != null)
                {
                    return prop.GetValue(this.obj);
                }
                MethodInfo method = this.obj.GetType().GetMethod(key, flags);
                if (method != null)
                {
                    return new JSObject(key, null, (object self, object[] args) => {
                        return method.Invoke(this.obj, args);
                    }, false);
                }
                return null;
            }

            public void Set(string key, object value)
            {
                BindingFlags flags = BindingFlags.Public | BindingFlags.NonPublic | BindingFlags.Instance;
                FieldInfo field = this.obj.GetType().GetField(key, flags);
                if (field != null)
                {
                    field.SetValue(this.obj, value);
                    return;
                }
                PropertyInfo prop = this.obj.GetType().GetProperty(key, flags);
                if (prop != null)
                {
                    prop.SetValue(this.obj, value);
                    return;
                }
                throw new Exception($"setting {key} on {obj} is forbidden");
            }

            public void Remove(string key)
            {
                throw new Exception($"removing ${key} on a DotNetInstanceValue is forbidden");
            }

            private object obj;
        }

        private class FS
        {
            public FS()
            {
                this.stdout = new Writer(Console.Out);
                this.stderr = new Writer(Console.Error);
            }

            public object Write(object self, object[] args)
            {
                var fd = (int)ToDouble(args[0]);
                var buf = (byte[])args[1];
                var offset = (int)ToDouble(args[2]);
                var length = (int)ToDouble(args[3]);
                var position = args[4];
                var callback = args[5];
                if (offset != 0 || length != buf.Length)
                {
                    ReflectApply(callback, null, new object[] { Enosys("write") });
                    return null;
                }
                if (position != null)
                {
                    ReflectApply(callback, null, new object[] { Enosys("write") });
                    return null;
                }
                int n = 0;
                switch (fd)
                {
                case 1:
                    this.stdout.Write(buf);
                    break;
                case 2:
                    this.stderr.Write(buf);
                    break;
                default:
                    ReflectApply(callback, null, new object[] { Enosys("write") });
                    break;
                }
                ReflectApply(callback, null, new object[] { null, buf.Length });
                return null;
            }

            private Writer stdout;
            private Writer stderr;
        }

        public static double? ToDouble(object value)
        {
            if (value == null)
            {
                return null;
            }

            switch (Type.GetTypeCode(value.GetType()))
            {
            case TypeCode.SByte:
                return (double)(sbyte)value;
            case TypeCode.Byte:
                return (double)(byte)value;
            case TypeCode.Int16:
                return (double)(short)value;
            case TypeCode.UInt16:
                return (double)(ushort)value;
            case TypeCode.Int32:
                return (double)(int)value;
            case TypeCode.UInt32:
                return (double)(uint)value;
            case TypeCode.Int64:
                return (double)(long)value;
            case TypeCode.UInt64:
                return (double)(ulong)value;
            case TypeCode.Single:
                return (double)(float)value;
            case TypeCode.Double:
                return (double)(double)value;
            case TypeCode.Decimal:
                return (double)(decimal)value;
            }
            return null;
        }

        public static JSObject Undefined = new JSObject("undefined");
        public static JSObject Global;

        static JSObject()
        {
            var rngCsp = new RNGCryptoServiceProvider();

            JSObject arr = new JSObject("Array");
            JSObject crypto = new JSObject("crypto", new Dictionary<string, object>()
            {
                {"getRandomValues", new JSObject((object self, object[] args) =>
                    {
                        var bs = (byte[])args[0];
                        rngCsp.GetBytes(bs);
                        return bs;
                    })},
            });
            JSObject obj = new JSObject("Object");
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

            FS fsimpl = new FS();
            JSObject fs = new JSObject("fs", new Dictionary<string, object>()
            {
                {"constants", new JSObject(new Dictionary<string, object>()
                    {
                        {"O_WRONLY", -1},
                        {"O_RDWR", -1},
                        {"O_CREAT", -1},
                        {"O_TRUNC", -1},
                        {"O_APPEND", -1},
                        {"O_EXCL", -1},
                    })},
                    {"write", new JSObject(fsimpl.Write)},
            });
            JSObject process = new JSObject("process");

            Global = new JSObject("global", new Dictionary<string, object>()
            {
                {"Array", arr},
                {"Object", obj},
                {"Uint8Array", u8},
                {"crypto", crypto},
                {"fs", fs},
                {"process", process},
                {".net", new JSObject(".net", new DotNetRootValues())},
            });
        }

        public static JSObject Go(IValues values)
        {
            return new JSObject("go", values);
        }

        public static JSObject Enosys(string name)
        {
            return new JSObject(new Dictionary<string, object>()
            {
                {"message", $"{name} not implemented"},
                {"code", "ENOSYS"},
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
            if (target is object[])
            {
                int idx = 0;
                if (int.TryParse(key, out idx))
                {
                    object[] arr = (object[])target;
                    return arr[idx];
                }
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

        public JSObject(string name)
            : this("", null, null, false)
        {
        }

        public JSObject(Dictionary<string, object> values)
            : this("", new DictionaryValues(values), null, false)
        {
        }

        public JSObject(string name, IValues values)
            : this(name, values, null, false)
        {
        }

        public JSObject(string name, Dictionary<string, object> values)
            : this(name, new DictionaryValues(values), null, false)
        {
        }

        public JSObject(JSFunc fn)
            : this("", null, fn, false)
        {
        }

        public JSObject(string name, IValues values, JSFunc fn, bool ctor)
        {
            const string defaultName = "(JSObject)";

            this.name = name;
            if (this.name == "")
            {
                this.name = defaultName;
            }
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
            if (this.values != null)
            {
                return this.values.Get(key);
            }
            throw new Exception($"{this}.{key} not found");
        }

        public void Set(string key, object value)
        {
            if (this.values == null)
            {
                this.values = new DictionaryValues(new Dictionary<string, object>());
            }
            this.values.Set(key, value);
        }

        public void Delete(string key)
        {
            if (this.values == null)
            {
                return;
            }
            this.values.Remove(key);
        }

        public object Invoke(object[] args)
        {
            if (this.fn == null)
            {
                throw new Exception($"{this} is not invokable since ${this} is not a function");
            }
            if (this.ctor)
            {
                throw new Exception($"{this} is not invokable since ${this} is a constructor");
            }
            return this.fn(null, args);
        }

        public override string ToString()
        {
            return this.name;
        }

        private IValues values;
        private string name;
        private JSFunc fn;
        private bool ctor = false;
    }`
