import { THittableCollections } from "@/types";

export const HITTABLE_METHODS = [
  "GET",
  "POST",
  "PUT",
  "PATCH",
  "DELETE",
  "HEAD",
];


export const METHOD_COLORS: Record<string, string> = {
  GET: "#00e5cc",
  POST: "#4ade80",
  PUT: "#fb923c",
  PATCH: "#a78bfa",
  DELETE: "#f87171",
  HEAD: "#94a3b8",
  OPTIONS: "#94a3b8",
};

export const collections: THittableCollections = [
  {
    collectionName: "JSONPlaceholder Lab",
    items: [
      {
        type: "route",
        name: "List Resources (GET)",
        curl: `curl -X GET https://jsonplaceholder.typicode.com/posts?_limit=5 \\
  -H "Accept: application/json"`,
        response: "",
      },
      {
        type: "route",
        name: "Create Resource (POST)",
        curl: `curl -X POST https://jsonplaceholder.typicode.com/posts \\
  -H "Content-Type: application/json" \\
  -d '{"title": "New Post", "body": "Testing proxy POST", "userId": 1}'`,
        response: "",
      },
      {
        type: "route",
        name: "Replace Resource (PUT)",
        curl: `curl -X PUT https://jsonplaceholder.typicode.com/posts/1 \\
  -H "Content-Type: application/json" \\
  -d '{"id": 1, "title": "Fully Updated", "body": "PUT replaces the whole resource", "userId": 1}'`,
        response: "",
      },
      {
        type: "route",
        name: "Update Resource (PATCH)",
        curl: `curl -X PATCH https://jsonplaceholder.typicode.com/posts/1 \\
  -H "Content-Type: application/json" \\
  -d '{"title": "Partially Updated"}'`,
        response: "",
      },
      {
        type: "route",
        name: "Remove Resource (DELETE)",
        curl: `curl -X DELETE https://jsonplaceholder.typicode.com/posts/1`,
        response: "",
      },
      {
        type: "route",
        name: "Check Headers (HEAD)",
        curl: `curl -I -X HEAD https://jsonplaceholder.typicode.com/posts`,
        response: "",
      },
    ],
    env: {
      host: "https://jsonplaceholder.typicode.com",
      token: "no-auth-required",
    },
  },
];
