package html

type AutoCapitalization string

const (
	AutoCapitalizationOn  AutoCapitalization = "on"
	AutoCapitalizationOff AutoCapitalization = "off"

	AutoCapitalizeNothing    AutoCapitalization = "none"
	AutoCapitalizeWords      AutoCapitalization = "words"
	AutoCapitalizeSentences  AutoCapitalization = "sentences"
	AutoCapitalizeCharacters AutoCapitalization = "characters"
)

type Editablility string

const (
	Editable            Editablility = "true"
	NotEditable         Editablility = "false"
	EditAsPlainTextOnly Editablility = "plaintext-only"
)

type Directionality string

const (
	LeftToRight        Directionality = "ltr"
	RightToLeft        Directionality = "rtl"
	AutomaticDirection Directionality = "auto"
)

type Bool string

const (
	True  Bool = "true"
	False Bool = "false"
)

type InputMode string

const (
	InputModeNone        InputMode = "none"
	InputModeText        InputMode = "text"
	InputModeDecimal     InputMode = "decimal"
	InputModeNumeric     InputMode = "numeric"
	InputModePhoneNumber InputMode = "tel"
	InputModeSearch      InputMode = "search"
	InputModeEmail       InputMode = "email"
	InputModeURL         InputMode = "url"
)

type YesNo string

const (
	Yes YesNo = "yes"
	No  YesNo = "no"
)

type VirtualKeyboardPolicy string

const (
	VirtualKeyboardAutomatic VirtualKeyboardPolicy = "auto"
	VirtualKeyboardManual    VirtualKeyboardPolicy = "manual"
)

type ReferrerPolicy string

const (
	NoReferrer                  ReferrerPolicy = "no-referrer"
	NoReferrerWhenDowngrade     ReferrerPolicy = "no-referrer-when-downgrade"
	Origin                      ReferrerPolicy = "origin"
	OriginWhenCrossOrigin       ReferrerPolicy = "origin-when-cross-origin"
	SameOrigin                  ReferrerPolicy = "same-origin"
	StrictOrigin                ReferrerPolicy = "strict-origin"
	StrictOriginWhenCrossOrigin ReferrerPolicy = "strict-origin-when-cross-origin"
	UnsafeURL                   ReferrerPolicy = "unsafe-url"
)

type Relationship string

const (
	RelationshipAlternate  Relationship = "alternate"
	RelationshipAuthor     Relationship = "author"
	RelationshipBookmark   Relationship = "bookmark"
	RelationshipExternal   Relationship = "external"
	RelationshipHelp       Relationship = "help"
	RelationshipLicense    Relationship = "license"
	RelationshipNext       Relationship = "next"
	RelationshipNoFollow   Relationship = "nofollow"
	RelationshipNoOpener   Relationship = "noopener"
	RelationshipNoReferrer Relationship = "noreferrer"
	RelationshipPrevious   Relationship = "prev"
	RelationshipSearch     Relationship = "search"
	RelationshipTag        Relationship = "tag"
)

type Target string

const (
	TargetBlank  Target = "_blank"
	TargetSelf   Target = "_self"
	TargetParent Target = "_parent"
	TargetTop    Target = "_top"
)

type ImageMapShape string

const (
	ImageMapRect    ImageMapShape = "rect"
	ImageMapCircle  ImageMapShape = "circle"
	ImageMapPoly    ImageMapShape = "poly"
	ImageMapDefault ImageMapShape = "default"
)

type CrossOrigin string

const (
	CrossOriginAnonymous      CrossOrigin = "anonymous"
	CrossOriginUseCredentials CrossOrigin = "use-credentials"
)

type Preload string

const (
	PreloadNone     Preload = "none"
	PreloadMetadata Preload = "metadata"
	PreloadAuto     Preload = "auto"
)

type FormMethod string

const (
	FormShouldGet         FormMethod = "get"
	FormShouldPost        FormMethod = "post"
	FormShouldCloseDialog FormMethod = "dialog"
)

type PopoverTargetAction string

const (
	PopoverHide   PopoverTargetAction = "hide"
	PopoverShow   PopoverTargetAction = "show"
	PopoverToggle PopoverTargetAction = "toggle"
)

type ButtonType string

const (
	ButtonSubmit ButtonType = "submit"
	ButtonReset  ButtonType = "reset"
	ButtonScript ButtonType = "button"
)

type CharacterSet string

const (
	UTF8 CharacterSet = "utf-8"
)

type OnOff string

const (
	On  OnOff = "on"
	Off OnOff = "off"
)

type Loading string

const (
	Eager Loading = "eager"
	Lazy  Loading = "lazy"
)

type Sandbox string

const (
	AllowDownloads                      Sandbox = "allow-downloads"
	AllowDownloadsWithUserActivation    Sandbox = "allow-downloads-with-user-activation"
	AllowForms                          Sandbox = "allow-forms"
	AllowModals                         Sandbox = "allow-modals"
	AllowOrientationLock                Sandbox = "allow-orientation-lock"
	AllowPointerLock                    Sandbox = "allow-pointer-lock"
	AllowPopups                         Sandbox = "allow-popups"
	AllowPopupsToEscapeSandbox          Sandbox = "allow-popups-to-escape-sandbox"
	AllowPresentation                   Sandbox = "allow-presentation"
	AllowSameOrigin                     Sandbox = "allow-same-origin"
	AllowScripts                        Sandbox = "allow-scripts"
	AllowTopNavigation                  Sandbox = "allow-top-navigation"
	AllowTopNavigationByUserActivation  Sandbox = "allow-top-navigation-by-user-activation"
	AllowTopNavigationToCustomProtocols Sandbox = "allow-top-navigation-to-custom-protocols"
)

type ImageDecodingStrategy string

const (
	DecodeImageSynchronously  ImageDecodingStrategy = "sync"
	DecodeImageAsynchronously ImageDecodingStrategy = "async"
	DecodeImageAutomatically  ImageDecodingStrategy = "auto"
)

type LinkType string

const (
	LinkedAudio    LinkType = "audio"
	LinkedDocument LinkType = "document"
	LinkedEmbed    LinkType = "embed"
	LinkedFetch    LinkType = "fetch"
	LinkedFont     LinkType = "font"
	LinkedImage    LinkType = "image"
	LinkedObject   LinkType = "object"
	LinkedScript   LinkType = "script"
	LinkedStyle    LinkType = "style"
	LinkedTrack    LinkType = "track"
	LinkedVideo    LinkType = "video"
	LinkedWorker   LinkType = "worker"
)

type ScriptType string

const (
	ScriptIsImportMap ScriptType = "importmap"
	ScriptIsModule    ScriptType = "module"
)

type Wrap string

const (
	WrapHard Wrap = "hard"
	WrapSoft Wrap = "soft"
)

type InputType string

const (
	InputButton        InputType = "button"
	InputCheckbox      InputType = "checkbox"
	InputColor         InputType = "color"
	InputDate          InputType = "date"
	InputDateTimeLocal InputType = "datetime-local"
	InputEmail         InputType = "email"
	InputFile          InputType = "file"
	InputHidden        InputType = "hidden"
	InputImage         InputType = "image"
	InputMonth         InputType = "month"
	InputNumber        InputType = "number"
	InputPassword      InputType = "password"
	InputRadio         InputType = "radio"
	InputRange         InputType = "range"
	InputReset         InputType = "reset"
	InputSearch        InputType = "search"
	InputSubmit        InputType = "submit"
	InputPhone         InputType = "tel"
	InputText          InputType = "text"
	InputTime          InputType = "time"
	InputURL           InputType = "url"
	InputWeek          InputType = "week"
)

type TableScope string

const (
	ScopeRow         TableScope = "row"
	ScopeColumn      TableScope = "col"
	ScopeRowGroup    TableScope = "rowgroup"
	ScopeColumnGroup TableScope = "colgroup"
)

type TrackKind string

const (
	TrackForSubtitles    TrackKind = "subtitles"
	TrackForCaptions     TrackKind = "captions"
	TrackForDescriptions TrackKind = "descriptions"
	TrackForChapters     TrackKind = "chapters"
	TrackForMetadata     TrackKind = "metadata"
)
