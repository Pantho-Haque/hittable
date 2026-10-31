import { NotificationContext } from "@/context/notifyContext";
import { TNotify } from "@/types/notification";
import { useContext } from "react";

export const useNotification = () => {
  const context = useContext(NotificationContext);
  if (context === undefined) {
    throw new Error(
      "useNotification must be used within a NotificationProvider",
    );
  }

  const { notify } = context;

  const info = ({ title, desc, position, timeout, action }: TNotify) => {
    notify({ title, desc, position, mode: "info", timeout, action });
  };

  const success = ({ title, desc, position, timeout, action }: TNotify) => {
    notify({ title, desc, position, mode: "success", timeout, action });
  };

  const error = ({ title, desc, position, timeout, action }: TNotify) => {
    notify({ title, desc, position, mode: "error", timeout, action });
  };

  return {
    notify,
    info,
    success,
    error,
  };
};
