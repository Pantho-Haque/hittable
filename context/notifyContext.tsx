"use client";
import {
  TNotificationContextType,
  TNotificationMode,
  TNotificationPosition,
  TNotificationType,
  TNotify,
} from "@/types";
import { AnimatePresence, MotionConfig, motion } from "framer-motion";
import { createContext, useState } from "react";

export const NotificationContext = createContext<
  TNotificationContextType | undefined
>(undefined);

export const NotificationProvider = ({
  children,
}: {
  children: React.ReactNode;
}) => {
  const [messages, setMessages] = useState<TNotificationType[]>([]);

  const notify = ({
    title,
    desc,
    position = "top-right",
    mode = "info", // Default to info if not provided
    timeout = 2000,
  }: TNotify) => {
    const id = Math.ceil(Math.random() * 10000000);
    const newMessage = {
      id,
      title,
      description: desc,
      position,
      mode,
      timeout,
    };

    setMessages((prev) => [...prev, newMessage]);

    setTimeout(() => {
      setMessages((prev) => prev.filter((msg) => msg.id !== id));
    }, timeout);
  };

  return (
    <NotificationContext.Provider value={{ notify }}>
      {messages.length > 0 &&
        Object.entries(
          messages.reduce((acc, msg) => {
            const position = msg.position || "top-right";
            if (!acc[position]) acc[position] = [];
            acc[position].push({ ...msg, mode: msg.mode || "info" });
            return acc;
          }, {} as Record<string, typeof messages>)
        ).map(([position, msgs]) => (
          <div
            key={position}
            className={`fixed z-999 ${positionClass(
              position as TNotificationPosition
            )}`}
            style={{
              margin: "1rem",
              display: "flex",
              flexDirection: "column",
              gap: "0.5rem",
            }}
          >
            {/* Follows the OS "reduce motion" setting. */}
            <MotionConfig reducedMotion="user">
            <AnimatePresence>
              {msgs.map((msg) => (
                <motion.div
                  initial={{
                    x: position.includes("right") ? 100 : -100,
                    opacity: 0,
                  }}
                  animate={{ x: 0, opacity: 1 }}
                  exit={{
                    x: position.includes("right") ? 100 : -100,
                    opacity: 0,
                  }}
                  transition={{ duration: 0.3, ease: "easeInOut" }}
                  key={msg.id}
                  className="rounded-md shadow-lg overflow-hidden truncate transition-all duration-300 transform max-w-[320px]"
                >
                  <div
                    className={`flex items-start p-4 border-l-4 ${modeClass(
                      msg.mode as TNotificationMode
                    )}`}
                  >
                    <div className="flex-1 ml-2">
                      <p className="font-medium ">
                        {msg.title}
                      </p>
                      <p className="text-sm truncate">
                        {msg.description}
                      </p>
                    </div>
                    <button
                      onClick={() =>
                        setMessages((prev) =>
                          prev.filter((m) => m.id !== msg.id)
                        )
                      }
                      className="ml-4"
                    >
                      ×
                    </button>
                  </div>
                </motion.div>
              ))}
            </AnimatePresence>
            </MotionConfig>
          </div>
        ))}
      {children}
    </NotificationContext.Provider>
  );
};

const positionClass = (position: TNotificationPosition) => {
  switch (position) {
    case "top-left":
      return "top-0 left-0";
    case "top-right":
      return "top-0 right-0";
    case "bottom-left":
      return "bottom-0 left-0";
    case "bottom-right":
      return "bottom-0 right-0";
  }
};

const modeClass = (mode: TNotificationMode) => {
  const textColor= 'text-zinc-700 dark:text-zinc-200'
  switch (mode) {
    case "info":
      return textColor + "border-l-blue-500 bg-blue-100 dark:bg-blue-300 ";
    case "success":
      return textColor + "border-l-green-500 bg-green-100 dark:bg-green-400";
    case "error":
      return textColor + "border-l-red-500 bg-red-100 dark:bg-red-400";
    default:
      return textColor + "border-l-blue-500 bg-blue-100 dark:bg-blue-300";
  }
};
