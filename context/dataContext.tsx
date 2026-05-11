import {
  createContext,
  useContext,
  useState,
  ReactNode,
  useEffect,
  SetStateAction,
  Dispatch,
  useCallback,
} from "react";
import {
  THittableCollections,
  THittableCurlJson,
  THittableSelectorResponse,
  TResponseJson,
} from "@/types";
import { GetHittableCollections } from "@/services";
import { useExtension } from "@/hooks/useExtension";
import { formatJson } from "@/utils/formatJson";
import { jsonToCurl } from "@/utils/curlConverter";
import { updateCurl } from "@/utils/hittableCollectionModifier";

const DataContext = createContext<{
  collections: THittableCollections;
  setCollections: Dispatch<SetStateAction<THittableCollections>>;
  selectorResponse: THittableSelectorResponse | null;
  setSelectorResponse: Dispatch<
    SetStateAction<THittableSelectorResponse | null>
  >;
  hasCollections: boolean;

  formInput: THittableCurlJson;
  setFormInput: Dispatch<SetStateAction<THittableCurlJson>>;
  proxyResponse: TResponseJson;
  setProxyResponse: Dispatch<SetStateAction<TResponseJson>>;
  isUnsaved: () => boolean;

  extensionAvailable: boolean;
  extensionChecked: boolean;

  handleSaveCollection: () => void;
} | null>(null);

export const DataProvider = ({ children }: { children: ReactNode }) => {
  const { available: extensionAvailable, checked: extensionChecked } =
    useExtension();
  const [collections, setCollections] = useState<THittableCollections>(
    GetHittableCollections(),
  );
  const [selectorResponse, setSelectorResponse] =
    useState<THittableSelectorResponse | null>({
      collectionName: "",
      curlName: "",
      env: {},
      curlJson: {
        url: "",
        method: "GET",
        headers: "",
        body: "",
        params: "",
      },
      responseJson: null,
    });

  const [formInput, setFormInput] = useState<THittableCurlJson>({
    url: selectorResponse?.curlJson?.url ?? "",
    method: selectorResponse?.curlJson?.method ?? "GET",
    headers: formatJson(selectorResponse?.curlJson?.headers ?? "").output,
    body: formatJson(selectorResponse?.curlJson?.body ?? "").output,
    params: selectorResponse?.curlJson?.params ?? "",
  });

  const [proxyResponse, setProxyResponse] = useState<TResponseJson>(
    selectorResponse?.responseJson ?? null,
  );

  const handleSaveCollection = useCallback(() => {
      setSelectorResponse({
        ...selectorResponse!,
        curlJson: formInput,
        responseJson: proxyResponse,
      });
      setCollections((prev) =>
        updateCurl(
          prev,
          selectorResponse!.collectionName,
          selectorResponse!.curlName,
          jsonToCurl(formInput),
          JSON.stringify(proxyResponse),
        ),
      );
    }, [selectorResponse, formInput, proxyResponse]);

  useEffect(() => {
    localStorage.setItem("hittable", JSON.stringify(collections));
  }, [collections]);

  const hasCollections = collections.length > 0;
  const isUnsaved = useCallback(() => {
    if (!selectorResponse || !formInput) return false;
    return jsonToCurl(selectorResponse.curlJson) !== jsonToCurl(formInput);
  }, [selectorResponse, formInput]);

  return (
    <DataContext.Provider
      value={{
        collections,
        setCollections,
        hasCollections,
        selectorResponse,
        setSelectorResponse,

        formInput,
        setFormInput,
        proxyResponse,
        setProxyResponse,
        isUnsaved,

        extensionAvailable,
        extensionChecked,

        handleSaveCollection,
      }}
    >
      {children}
    </DataContext.Provider>
  );
};

export const useDataContext = () => {
  const context = useContext(DataContext);
  if (!context) {
    throw new Error("useDataContext must be used within a DataProvider");
  }
  return context;
};
