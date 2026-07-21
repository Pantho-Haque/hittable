export * from "./notification";
export * from "./hittable";
export * from "./note";
export * from "./workspace";

export type TApiResponse<T> = {
  data?: T;
  message: string;
  ok: boolean;
  status: number;
};