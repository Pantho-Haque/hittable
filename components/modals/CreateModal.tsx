"use client";

import { Plus } from "lucide-react";
import { useState } from "react";
import {
  THittableSelectorSelection,
} from "@/types";
import {
  createCollectionName,
  createCurlName,
  isAlreadyExistsInPath,
} from "@/utils/hittableCollectionModifier";
import { ModalInput, ModalShell, ModalActions } from "@/components";
import { useDataContext } from "@/context/dataContext";


export default function CreateModal({
  type,
  selection,
  setSelection,
  collectionCurlList,
  folderPath,
}: {
  type: "collection" | "route";
  selection: THittableSelectorSelection;
  setSelection: (value: THittableSelectorSelection) => void;
  collectionCurlList: { [key: string]: string[] };
  folderPath?: string[];
}) {
  const [open, setOpen] = useState(false);
  const [value, setValue] = useState("");
  const [curlString, setCurlString] = useState("");
  const [error, setError] = useState("");
  const { setSelectorResponse, setCollections, collections } = useDataContext();

  const handleCreate = () => {
    if (!value.trim()) return setOpen(false);
    let newName = value.trim();

    if (type === "route") {
      const col = collections.find(
        (c) => c.collectionName === selection.collectionName,
      );
      if (isAlreadyExistsInPath(col, folderPath ?? [], newName)) {
        newName += " - New";
      }
    } else {
      if (collectionCurlList[newName]) {
        newName += " - New";
      }
    }

    if (type === "collection") {
      setCollections((prev) => createCollectionName(prev, newName));
      setSelection({ collectionName: newName, folderPath: [], curlName: "" });
      setSelectorResponse(null);
    } else {
      setCollections((prev) =>
        createCurlName(
          prev,
          selection.collectionName,
          newName,
          curlString,
          folderPath,
        ),
      );
      setSelection({
        collectionName: selection.collectionName,
        folderPath: folderPath ?? [],
        curlName: newName,
      });
    }
    setValue("");
    setCurlString("");
    setOpen(false);
  };


  return (
    <>
      <button
        onClick={(e) => {
          e.stopPropagation();
          setOpen(true);
        }}
        title={type === "collection" ? "New Collection" : "New Route"}
        className="modal-button-mini"
      >
        <Plus size={14} />
      </button>

      {open && (
        <ModalShell
          title={`Create ${type}`}
          subtitle={
            type === "route"
              ? `Adding to ${selection.collectionName}${folderPath?.length ? " / " + folderPath.join(" / ") : ""}`
              : "Start a new collection of routes"
          }
          onClose={() => setOpen(false)}
        >
          {error && <p className="text-[10px] text-red-400 -mt-2">{error}</p>}
          <ModalInput
            autoFocus
            value={value}
            onChange={(v) => {
              setValue(v);
              setError("");
            }}
            placeholder={type === "collection" ? "my-api" : "get-users"}
            onKeyDown={(e) => {
              if (e.key === "Enter") handleCreate();
              if (e.key === "Escape") setOpen(false);
            }}
          />
          {type === "route" && (
            <textarea
              className="w-full h-24 p-2 text-xs  rounded-md bg-white/5 text-white focus:outline-none focus:border-cyan-500/50 focus:ring-1 focus:ring-cyan-500/50"
              value={curlString}
              onChange={(e) => {
                setCurlString(e.target.value);
                setError("");
              }}
              placeholder={"Paste you curl here"}
              onKeyDown={(e) => {
                if (e.key === "Enter") handleCreate();
                if (e.key === "Escape") setOpen(false);
              }}
            />
          )}
          <ModalActions
            onCancel={() => setOpen(false)}
            onConfirm={handleCreate}
            confirmLabel="Create"
          />
        </ModalShell>
      )}
    </>
  );
}
