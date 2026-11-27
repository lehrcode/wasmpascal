{ Unit Dos von WasmPascal (PLAN.md, Etappen 7 und 15).

  Datum und Uhrzeit kommen aus der Uhr des Ziels (System._Clock) und gelten
  in Ortszeit: Den Versatz zu UTC liest die Unit beim Start aus der
  Umgebungsvariable WASMPASCAL_UTC_OFFSET (Minuten oestlich von UTC; pascal
  run und web/loader.js setzen sie). Fehlt sie, gilt UTC. Der Versatz bleibt
  fuer den ganzen Lauf gleich.

  Dateiattribute: Nur Directory und Hidden (Name beginnt mit einem Punkt)
  haben eine Wirkung; ReadOnly, SysFile, VolumeID und Archive werden nie
  gemeldet und beim Suchen ignoriert. Pfade sind wie in der System-Unit
  relativ zum aktuellen Verzeichnis innerhalb der Wurzel; "\" gilt als "/".
  PathStr usw. und SearchRec.Name haben 255 Bytes statt der TP-Laengen. }
unit Dos;

interface

const
  { Dateiattribute }
  ReadOnly = $01;
  Hidden = $02;
  SysFile = $04;
  VolumeID = $08;
  Directory = $10;
  Archive = $20;
  AnyFile = $3F;

type
  PathStr = string;
  DirStr = string;
  NameStr = string;
  ExtStr = string;

  DateTime = record
    Year, Month, Day, Hour, Min, Sec: Word
  end;

  SearchRec = record
    Fill: array[1..21] of Byte;   { intern: Zeiger auf den Suchzustand }
    Attr: Byte;
    Time: LongInt;
    Size: LongInt;
    Name: string
  end;

var
  DosError: Integer;   { 0, 2 Datei fehlt, 3 Pfad fehlt, 6 nicht geoeffnet, 18 keine weiteren Dateien }

procedure GetDate(var Year, Month, Day, DayOfWeek: Word);
procedure GetTime(var Hour, Minute, Second, Sec100: Word);
procedure GetFAttr(var F; var Attr: Word);
procedure GetFTime(var F; var Time: LongInt);
procedure FindFirst(Path: PathStr; Attr: Word; var S: SearchRec);
procedure FindNext(var S: SearchRec);
function FExpand(Path: PathStr): PathStr;
procedure FSplit(Path: PathStr; var Dir: DirStr; var Name: NameStr; var Ext: ExtStr);
function FSearch(Path: PathStr; DirList: string): PathStr;
function GetEnv(EnvVar: string): string;
function EnvCount: Integer;
function EnvStr(Index: Integer): string;
function DosVersion: Word;
procedure PackTime(var T: DateTime; var P: LongInt);
procedure UnpackTime(P: LongInt; var T: DateTime);

implementation

var
  OffsetMin: LongInt;   { Versatz der Ortszeit zu UTC in Minuten }

{ ----------------------------------------------------------- Zeit }

{ Ortszeit in Nanosekunden aus UTC-Nanosekunden }
function Local(Ns: Real): Real;
begin
  Local := Ns + OffsetMin * 60E9
end;

{ Tage und Hundertstelsekunden des Tages seit dem 1. 1. 1970. Ns hat als
  Real nur eine Genauigkeit von etwa 256 ns (53 Bit bei rund 1,8e18); der
  Bruchteil der Sekunde bekommt deshalb 1 µs Toleranz nach oben, damit z. B.
  0,25 s nicht als 0,2499999 s zu 24 Hundertsteln wird. }
procedure Zerlege(Ns: Real; var Tage, Hundertstel: LongInt);
var
  Sek: Real;
  H: LongInt;
begin
  Sek := Int(Ns / 1E9);
  Tage := Trunc(Int(Sek / 86400));
  if Sek < Tage * 86400.0 then   { vor 1970: abrunden }
    Dec(Tage);
  H := Trunc((Ns - Sek * 1E9 + 1000) / 1E7);
  if H < 0 then
    H := 0
  else if H > 99 then
    H := 99;
  Hundertstel := Trunc((Sek - Tage * 86400.0) * 100) + H
end;

{ Datum zu einem Tag seit dem 1. 1. 1970, nach H. Hinnant, "civil_from_days" }
procedure Kalender(Tage: LongInt; var Year, Month, Day, DayOfWeek: Word);
var
  Z, Era, Doe, Yoe, Doy, Mp, Y, M: LongInt;
begin
  DayOfWeek := ((Tage mod 7) + 11) mod 7;   { 1. 1. 1970 war ein Donnerstag; 0 = Sonntag }
  Z := Tage + 719468;
  if Z >= 0 then
    Era := Z div 146097
  else
    Era := (Z - 146096) div 146097;
  Doe := Z - Era * 146097;
  Yoe := (Doe - Doe div 1460 + Doe div 36524 - Doe div 146096) div 365;
  Y := Yoe + Era * 400;
  Doy := Doe - (365 * Yoe + Yoe div 4 - Yoe div 100);
  Mp := (5 * Doy + 2) div 153;
  Day := Doy - (153 * Mp + 2) div 5 + 1;
  if Mp < 10 then
    M := Mp + 3
  else
    M := Mp - 9;
  if M <= 2 then
    Inc(Y);
  Year := Y;
  Month := M
end;

procedure GetDate(var Year, Month, Day, DayOfWeek: Word);
var
  Tage, H: LongInt;
begin
  Zerlege(Local(_Clock(0)), Tage, H);
  Kalender(Tage, Year, Month, Day, DayOfWeek)
end;

procedure GetTime(var Hour, Minute, Second, Sec100: Word);
var
  Tage, H: LongInt;
begin
  Zerlege(Local(_Clock(0)), Tage, H);
  Hour := H div 360000;
  Minute := H div 6000 mod 60;
  Second := H div 100 mod 60;
  Sec100 := H mod 100
end;

procedure PackTime(var T: DateTime; var P: LongInt);
begin
  P := (LongInt(T.Year - 1980) shl 25) or (LongInt(T.Month) shl 21) or (LongInt(T.Day) shl 16) or
    (LongInt(T.Hour) shl 11) or (LongInt(T.Min) shl 5) or (LongInt(T.Sec) shr 1)
end;

procedure UnpackTime(P: LongInt; var T: DateTime);
begin
  T.Year := ((P shr 25) and $7F) + 1980;
  T.Month := (P shr 21) and $0F;
  T.Day := (P shr 16) and $1F;
  T.Hour := (P shr 11) and $1F;
  T.Min := (P shr 5) and $3F;
  T.Sec := (P and $1F) * 2
end;

{ Dateizeit (UTC-Nanosekunden) als gepackte DOS-Zeit in Ortszeit.
  TP-ANNAHME: Zeiten vor 1980 bzw. nach 2107 werden auf die Grenzen des
  DOS-Formats gesetzt. }
function PackNs(Ns: Real): LongInt;
var
  Tage, H: LongInt;
  T: DateTime;
  W: Word;
  P: LongInt;
begin
  Zerlege(Local(Ns), Tage, H);
  Kalender(Tage, T.Year, T.Month, T.Day, W);
  T.Hour := H div 360000;
  T.Min := H div 6000 mod 60;
  T.Sec := H div 100 mod 60;
  if T.Year < 1980 then
  begin
    T.Year := 1980; T.Month := 1; T.Day := 1;
    T.Hour := 0; T.Min := 0; T.Sec := 0
  end
  else if T.Year > 2107 then
  begin
    T.Year := 2107; T.Month := 12; T.Day := 31;
    T.Hour := 23; T.Min := 59; T.Sec := 58
  end;
  PackTime(T, P);
  PackNs := P
end;

{ ------------------------------------------------------ Umgebung }

function EnvCount: Integer;
begin
  EnvCount := _EnvCount
end;

function EnvStr(Index: Integer): string;
begin
  EnvStr := _EnvStr(Index)
end;

function Gross(const S: string): string;
var
  I: Integer;
begin
  Result := S;
  for I := 1 to Length(Result) do
    Result[I] := UpCase(Result[I])
end;

{ TP-ANNAHME: zuerst der exakte Name, dann ohne Ruecksicht auf Gross-/
  Kleinschreibung (DOS speicherte die Namen gross). }
function GetEnv(EnvVar: string): string;
var
  I, P: Integer;
  S, Ersatz: string;
  Gefunden: Boolean;
begin
  Ersatz := '';
  Gefunden := False;
  for I := 1 to _EnvCount do
  begin
    S := _EnvStr(I);
    P := Pos('=', S);
    if P = 0 then
      P := Length(S) + 1;
    if Copy(S, 1, P - 1) = EnvVar then
    begin
      GetEnv := Copy(S, P + 1, 255);
      Exit
    end;
    if not Gefunden and (Gross(Copy(S, 1, P - 1)) = Gross(EnvVar)) then
    begin
      Ersatz := Copy(S, P + 1, 255);
      Gefunden := True
    end
  end;
  GetEnv := Ersatz
end;

function DosVersion: Word;
begin
  DosVersion := 5   { Lo = 5 (Hauptversion), Hi = 0 }
end;

{ --------------------------------------------------------- Pfade }

function FExpand(Path: PathStr): PathStr;
begin
  FExpand := _FullPath(Path)
end;

procedure FSplit(Path: PathStr; var Dir: DirStr; var Name: NameStr; var Ext: ExtStr);
var
  I, P: Integer;
begin
  I := Length(Path);
  while (I > 0) and (Path[I] <> '/') and (Path[I] <> '\') do
    Dec(I);
  Dir := Copy(Path, 1, I);
  Name := Copy(Path, I + 1, 255);
  P := Length(Name);
  while (P > 0) and (Name[P] <> '.') do
    Dec(P);
  if P > 0 then
  begin
    Ext := Copy(Name, P, 255);
    Name := Copy(Name, 1, P - 1)
  end
  else
    Ext := ''
end;

function IstDatei(const P: string): Boolean;
var
  Kind, Size: LongInt;
  M: Real;
begin
  IstDatei := (_FileStat(P, Kind, Size, M) = 0) and (Kind = 1)
end;

function FSearch(Path: PathStr; DirList: string): PathStr;
var
  I, J: Integer;
  D, P: string;
begin
  FSearch := '';
  if IstDatei(Path) then
  begin
    FSearch := Path;
    Exit
  end;
  I := 1;
  while I <= Length(DirList) do
  begin
    J := I;
    while (J <= Length(DirList)) and (DirList[J] <> ';') do
      Inc(J);
    D := Copy(DirList, I, J - I);
    I := J + 1;
    if D = '' then
      Continue;
    if (D[Length(D)] = '/') or (D[Length(D)] = '\') then
      P := D + Path
    else
      P := D + '/' + Path;
    if IstDatei(P) then
    begin
      FSearch := P;
      Exit
    end
  end
end;

{ ------------------------------------------------- Attribute, Zeit }

{ Letzter Teil eines Pfads (nach dem letzten "/" oder "\") }
function LetzterTeil(const P: string): string;
var
  I: Integer;
begin
  I := Length(P);
  while (I > 0) and (P[I] <> '/') and (P[I] <> '\') do
    Dec(I);
  LetzterTeil := Copy(P, I + 1, 255)
end;

function Attribute(const Name: string; Kind: LongInt): Byte;
var
  A: Byte;
begin
  A := 0;
  if Kind = 2 then
    A := Directory;
  if (Length(Name) > 0) and (Name[1] = '.') then
    A := A or Hidden;
  Attribute := A
end;

procedure GetFAttr(var F; var Attr: Word);
var
  Kind, Size: LongInt;
  M: Real;
  N: string;
begin
  N := _FileName(F);
  DosError := _FileStat(N, Kind, Size, M);
  if DosError = 0 then
    Attr := Attribute(LetzterTeil(_FullPath(N)), Kind)
  else
    Attr := 0
end;

procedure GetFTime(var F; var Time: LongInt);
var
  Kind, Size: LongInt;
  M: Real;
begin
  DosError := _FileStatF(F, Kind, Size, M);
  if DosError = 0 then
    Time := PackNs(M)
end;

{ ------------------------------------------------------ Suchen }

type
  PSuche = ^TSuche;
  TSuche = record
    Handle, Cookie: LongInt;
    Attr: Word;
    Muster: string;
    Dir: string
  end;
  PPSuche = ^PSuche;

{ Passt Name auf Muster mit * und ?, ohne Ruecksicht auf Gross-/
  Kleinschreibung (wie DOS). ? steht fuer ein Byte. }
function Passt(const Muster, Name: string): Boolean;
var
  M, N, StarM, StarN: Integer;
begin
  M := 1;
  N := 1;
  StarM := 0;
  StarN := 0;
  while N <= Length(Name) do
  begin
    if (M <= Length(Muster)) and ((Muster[M] = '?') or (UpCase(Muster[M]) = UpCase(Name[N]))) then
    begin
      Inc(M);
      Inc(N)
    end
    else if (M <= Length(Muster)) and (Muster[M] = '*') then
    begin
      StarM := M;
      StarN := N;
      Inc(M)
    end
    else if StarM > 0 then
    begin
      M := StarM + 1;
      Inc(StarN);
      N := StarN
    end
    else
    begin
      Passt := False;
      Exit
    end
  end;
  while (M <= Length(Muster)) and (Muster[M] = '*') do
    Inc(M);
  Passt := M > Length(Muster)
end;

{ TP-ANNAHME: Wie unter DOS passt "*.*" (allgemein ein Muster, das auf ".*"
  endet) auch auf Namen ohne Punkt. }
function PasstDos(const Muster, Name: string): Boolean;
var
  L: Integer;
begin
  L := Length(Muster);
  if Passt(Muster, Name) then
    PasstDos := True
  else if (L >= 2) and (Muster[L - 1] = '.') and (Muster[L] = '*') and (Pos('.', Name) = 0) then
    PasstDos := Passt(Copy(Muster, 1, L - 2), Name)
  else
    PasstDos := False
end;

function Zustand(var S: SearchRec): PPSuche;
begin
  Zustand := PPSuche(@S.Fill)
end;

procedure FindNext(var S: SearchRec);
var
  Z: PSuche;
  Name: string;
  Kind, Size: LongInt;
  M: Real;
  A: Byte;
begin
  Z := Zustand(S)^;
  DosError := 18;
  if Z = nil then
    Exit;
  while _DirNext(Z^.Handle, Z^.Cookie, Name, Kind) do
  begin
    if (Name = '.') or (Name = '..') or not PasstDos(Z^.Muster, Name) then
      Continue;
    if _FileStat(Z^.Dir + '/' + Name, Kind, Size, M) <> 0 then
      Continue;
    A := Attribute(Name, Kind);
    if (A and Directory <> 0) and (Z^.Attr and Directory = 0) then
      Continue;
    if (A and Hidden <> 0) and (Z^.Attr and Hidden = 0) then
      Continue;
    S.Attr := A;
    S.Time := PackNs(M);
    if Kind = 2 then
      S.Size := 0
    else
      S.Size := Size;
    S.Name := Name;
    DosError := 0;
    Exit
  end;
  { Ende: Zustand freigeben }
  _DirClose(Z^.Handle);
  Dispose(Z);
  Zustand(S)^ := nil
end;

procedure FindFirst(Path: PathStr; Attr: Word; var S: SearchRec);
var
  I: Integer;
  Z: PSuche;
  H: LongInt;
  Dir: string;
begin
  Zustand(S)^ := nil;
  I := Length(Path);
  while (I > 0) and (Path[I] <> '/') and (Path[I] <> '\') do
    Dec(I);
  Dir := Copy(Path, 1, I - 1);
  if (I = 1) then
    Dir := '/'
  else if I = 0 then
    Dir := '.';
  DosError := _DirOpen(Dir, H);
  if DosError <> 0 then
    Exit;
  New(Z);
  Z^.Handle := H;
  Z^.Cookie := 0;
  Z^.Attr := Attr;
  Z^.Muster := Copy(Path, I + 1, 255);
  Z^.Dir := Dir;
  Zustand(S)^ := Z;
  FindNext(S)
end;

var
  Code: Integer;

begin
  DosError := 0;
  Val(GetEnv('WASMPASCAL_UTC_OFFSET'), OffsetMin, Code);
  if Code <> 0 then
    OffsetMin := 0
end.
