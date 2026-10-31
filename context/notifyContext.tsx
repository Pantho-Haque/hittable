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
    action,
  }: TNotify) => {
    const id = Math.ceil(Math.random() * 10000000);
    const newMessage = {
      id,
      title,
      description: desc,
      position,
      mode,
      timeout,
      action,
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
                  className="rounded-md shadow-lg shadow-black/40 overflow-hidden max-w-[340px]"
                >
                  {/* Errors interrupt (alert); everything else waits its turn. */}
                  <div
                    role={msg.mode === "error" ? "alert" : "status"}
                    aria-live={msg.mode === "error" ? "assertive" : "polite"}
                    className={`flex items-start gap-3 py-3 pl-3 pr-2 border border-white/10 border-l-4 bg-(--surface-2) text-white/90 ${modeClass(
                      msg.mode as TNotificationMode
                    )}`}
                  >
                    <div className="flex-1 min-w-0">
                      <p className="text-sm font-semibold">{msg.title}</p>
                      {msg.description && (
                        <p className="text-xs text-white/70 break-words">{msg.description}</p>
                      )}
                    </div>
                    {msg.action && (
                      <button
                        type="button"
                        onClick={() => {
                          msg.action!.onClick();
                          setMessages((prev) => prev.filter((m) => m.id !== msg.id));
                        }}
                        className="shrink-0 self-center rounded border border-white/15 px-2 py-1 text-xs font-semibold text-(--brand) hover:bg-white/5 cursor-pointer"
                      >
                        {msg.action.label}
                      </button>
                    )}
                    <button
                      type="button"
                      aria-label="Dismiss notification"
                      onClick={() =>
                        setMessages((prev) =>
                          prev.filter((m) => m.id !== msg.id)
                        )
                      }
                      className="shrink-0 rounded px-1.5 text-base leading-none text-white/60 hover:text-white cursor-pointer"
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

// Toasts sit on the app's own dark surface whatever the OS theme is; the
// mode shows as the left border colour (and the role says it in words).
const modeClass = (mode: TNotificationMode) => {
  switch (mode) {
    case "success":
      return "border-l-(--success)";
    case "error":
      return "border-l-(--danger)";
    default:
      return "border-l-(--info)";
  }
};
