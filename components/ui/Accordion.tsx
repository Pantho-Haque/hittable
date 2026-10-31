"use client"

import * as React from "react"
import * as AccordionPrimitive from "@radix-ui/react-accordion"
import { ChevronDownIcon } from "lucide-react"
import { cn } from "@/utils/cn"

export function Accordion({
  ...props
}: React.ComponentProps<typeof AccordionPrimitive.Root>) {
  return <AccordionPrimitive.Root data-slot="accordion" {...props} />
}

export function AccordionItem({
  className,
  ...props
}: React.ComponentProps<typeof AccordionPrimitive.Item>) {
  return (
    <AccordionPrimitive.Item
      data-slot="accordion-item"
      className={cn(
        "border-b border-white/5 last:border-b-0",
        className
      )}
      {...props}
    />
  )
}

export function AccordionTrigger({
  className,
  children,
  ...props
}: React.ComponentProps<typeof AccordionPrimitive.Trigger>) {
  return (
    <AccordionPrimitive.Header className="flex">
      <AccordionPrimitive.Trigger
        data-slot="accordion-trigger"
        className={cn(
          "flex flex-1 items-center justify-between gap-4 p-3 text-left text-xs font-semibold tracking-wide text-white/50 transition-all outline-none cursor-pointer hover:text-cyan-400 hover:bg-white/5 data-[state=open]:text-cyan-400 [&[data-state=open]>svg]:rotate-180 disabled:pointer-events-none disabled:opacity-50",
          className
        )}
        {...props}
      >
        {children}
        <ChevronDownIcon className="pointer-events-none size-3.5 shrink-0 text-white/50 transition-transform duration-200 data-[state=open]:text-cyan-400" />
      </AccordionPrimitive.Trigger>
    </AccordionPrimitive.Header>
  )
}

export function AccordionContent({
  className,
  children,
  ...props
}: React.ComponentProps<typeof AccordionPrimitive.Content>) {
  return (
    <AccordionPrimitive.Content
      data-slot="accordion-content"
      className="data-[state=closed]:animate-accordion-up data-[state=open]:animate-accordion-down overflow-hidden text-xs px-7"
      {...props}
    >
      <div className={cn("pb-3 pt-0", className)}>{children}</div>
    </AccordionPrimitive.Content>
  )
}
