// SPDX-License-Identifier: Apache-2.0

using System;
using Go2DotNet.Example.MonoGame.AutoGen;

namespace Go2DotNet.Example.MonoGame
{
    public static class Program
    {
        [STAThread]
        static void Main()
        {
            Go go = new Go();
            go.Run();
        }
    }
}
