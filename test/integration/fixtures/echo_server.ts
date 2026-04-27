#!/usr/bin/env -S npx tsx
import * as readline from "node:readline";

const TOOLS = [
  {
    name: "echo",
    description: "Echo back the input text",
    inputSchema: {
      type: "object",
      properties: {
        text: { type: "string", description: "Text to echo" },
      },
      required: ["text"],
    },
  },
  {
    name: "add",
    description: "Add two numbers",
    inputSchema: {
      type: "object",
      properties: {
        a: { type: "number", description: "First number" },
        b: { type: "number", description: "Second number" },
      },
      required: ["a", "b"],
    },
  },
];

function handleRequest(req: any): any {
  const reqId = req.id;
  const method: string = req.method;
  const params: any = req.params || {};

  if (method === "tools/list") {
    return { jsonrpc: "2.0", id: reqId, result: { tools: TOOLS } };
  }

  if (method === "tools/call") {
    const name: string = params.name || "";
    const args: any = params.arguments || {};

    if (name === "echo") {
      const text: string = args.text || "";
      return {
        jsonrpc: "2.0",
        id: reqId,
        result: { content: [{ type: "text", text: `Echo: ${text}` }] },
      };
    }

    if (name === "add") {
      const a: number = args.a || 0;
      const b: number = args.b || 0;
      return {
        jsonrpc: "2.0",
        id: reqId,
        result: { content: [{ type: "text", text: String(a + b) }] },
      };
    }

    return {
      jsonrpc: "2.0",
      id: reqId,
      error: { code: -32601, message: `Tool not found: ${name}` },
    };
  }

  return {
    jsonrpc: "2.0",
    id: reqId,
    error: { code: -32601, message: `Method not found: ${method}` },
  };
}

const rl = readline.createInterface({ input: process.stdin });
rl.on("line", (line: string) => {
  line = line.trim();
  if (!line) return;
  try {
    const req = JSON.parse(line);
    const resp = handleRequest(req);
    process.stdout.write(JSON.stringify(resp) + "\n");
  } catch (err: any) {
    process.stderr.write(`JSON parse error: ${err.message}\n`);
  }
});
