// SPDX-License-Identifier: Apache-2.0

using System;
using Go2DotNet.Example.Goroutines.AutoGen;

namespace Go2DotNet.Example.Goroutines
{
    class Program
    {
        static void Main(string[] args)
        {
            Go go = new Go();
            go.Run().Wait();
        }
    }    
}
