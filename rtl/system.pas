{ System-Unit von WasmPascal.

  Sie wird implizit vor jedem Programm uebersetzt. Routinen, deren Name mit
  einem Unterstrich beginnt, ruft der Compiler fuer Sprachmittel und
  Standardroutinen auf; Anwenderprogramme benutzen sie nicht direkt.

  Intrinsics des Compilers:
    _HeapBase       erste Adresse hinter dem Schatten-Stack
    _MemSize        Speichergroesse in Seiten zu 64 KB
    _MemGrow(n)     memory.grow, liefert -1 bei Fehlschlag
    _RunError(n)    Laufzeitfehler n an der Stelle des Aufrufers
    _FdWrite usw.   WASI-Aufrufe (compiler/internal/abi), Ergebnis: errno
    _GoWasmWrite    Importe aus wasm_exec.js fuer das Ziel gojs (Go-Stack sp)
    _Suspend(g, a)  Anhaltepunkt des Schedulers

  Nur die "unterste Schicht" unterscheidet sich zwischen den Zielen; sie ist
  per bedingter Uebersetzung (Symbole WASI bzw. GOJS) getrennt.

  Laufzeitroutinen setzen die vorgemerkte Quelltextstelle nicht; Fehler darin
  nennen die Anweisung des Anwenderprogramms. }
unit System;

interface

var
  Input, Output: Text;
  RandSeed: LongInt;
  FileMode: Byte;
  ExitCode: Integer;

{ Uhr in Nanosekunden: Id 0 = Echtzeit seit 1. 1. 1970 (UTC), 1 = monoton. }
function _Clock(Id: LongInt): Real;

implementation

type
  PChar = ^Char;
  PLongInt = ^LongInt;
  PFreeBlock = ^TFreeBlock;
  TFreeBlock = record
    Next: PFreeBlock;
    Size: LongInt
  end;
  TSetBytes = array[0..31] of Byte;

  { Laufzeit-Records der Dateien. Handle, Mode und Name liegen in beiden an
    denselben Offsets. Die Groessen muessen zu types.FileRecSize und
    types.TextRecSize im Compiler passen. }
  TFileRec = record
    Handle: LongInt;   { WASI-Dateideskriptor }
    Mode: LongInt;     { fmClosed, fmInput, fmOutput, fmInOut }
    RecSize: LongInt;  { Satzgroesse typisierter und untypisierter Dateien }
    Reserved: LongInt;
    Name: string
  end;
  TTextRec = record
    Handle: LongInt;
    Mode: LongInt;
    BufPos: LongInt;   { naechstes Zeichen im Puffer }
    BufEnd: LongInt;   { Ende der gelesenen Zeichen }
    Name: string;
    Buffer: array[0..255] of Char
  end;

const
  { Obergrenze des Heaps, damit Adressen als LongInt positiv bleiben. }
  HeapLimit = $7FFF0000;
  PageSize = 65536;

  _FileRecSize = SizeOf(TFileRec);
  _TextRecSize = SizeOf(TTextRec);

  { Dateimodi wie in TP }
  fmClosed = $D7B0;
  fmInput = $D7B1;
  fmOutput = $D7B2;
  fmInOut = $D7B3;

  BufSize = 256;
  RootFd = 3;  { erstes vorgeoeffnetes Verzeichnis (der Runner bindet es ein) }

  { WASI-Rechte: fd_read, fd_seek, fd_tell, fd_filestat_get; zum Schreiben
    zusaetzlich fd_datasync, fd_sync, fd_write, fd_filestat_set_size }
  RightsRead = $200026;
  RightsWrite = $600077;

  { Gruende fuer _Suspend (Scheduler, PLAN.md) }
  SuspendYield = 1;   { Zeitscheibe bei Ausgaben abgelaufen }
  SuspendDelay = 2;   { Delay, Argument in Millisekunden }
  SuspendInput = 3;   { Eingabe gebraucht }
  SuspendHalt = 4;    { Programmende, Argument: Exit-Code }
  YieldMs = 30;

var
  InOutRes: Integer;     { Fehler der letzten E/A, bis IOResult }
  CurDir: string;        { aktuelles Verzeichnis relativ zur Wurzel, ohne / }
  IoVec: record
    Buf, Len: LongInt
  end;
  IoCount: LongInt;
  ArgCount, ArgPtrs, ArgBuf: LongInt;
  ArgStride: LongInt;    { Abstand der argv-Eintraege: 4 (WASI) bzw. 8 (gojs) }
  LastYield: Real;       { Zeitpunkt des letzten Anhaltens in ms }
  HeapPtr: LongInt;      { erste Adresse oberhalb aller vergebenen Bloecke }
  HeapEnd: LongInt;      { Ende des vorhandenen Speichers }
  FreeList: PFreeBlock;  { freie Bloecke unterhalb von HeapPtr, nach Adresse sortiert }

{ ------------------------------------------------------------------ Heap }

{ Bloecke haben keinen Kopf: Wie in TP gibt der Aufrufer bei der Freigabe die
  Groesse an. Groessen werden auf 8 Byte aufgerundet; ein freier Block traegt
  Nachfolger und Groesse in seinen ersten 8 Byte. }

function _Round8(Size: LongInt): LongInt;
begin
  if Size <= 0 then
    Size := 1;
  _Round8 := (Size + 7) and not 7
end;

{ meldet dem Host vergroesserten Speicher (unterste Schicht) }
procedure MemGrown; forward;

function _GetMem(Size: LongInt): Pointer;
var
  Prev, Cur, Rest: PFreeBlock;
  Need: LongInt;
begin
  _GetMem := nil;
  Size := _Round8(Size);
  { first fit in der Freiliste }
  Prev := nil;
  Cur := FreeList;
  while Cur <> nil do
  begin
    if Cur^.Size >= Size then
    begin
      if Cur^.Size - Size >= 8 then
      begin
        Rest := PFreeBlock(LongInt(Cur) + Size);
        Rest^.Next := Cur^.Next;
        Rest^.Size := Cur^.Size - Size
      end
      else
        Rest := Cur^.Next;
      if Prev = nil then
        FreeList := Rest
      else
        Prev^.Next := Rest;
      _GetMem := Cur;
      Exit
    end;
    Prev := Cur;
    Cur := Cur^.Next
  end;
  { sonst oben anfuegen, bei Bedarf Speicher nachfordern }
  if Size > HeapLimit - HeapPtr then
    Exit;
  if HeapPtr + Size > HeapEnd then
  begin
    Need := (HeapPtr + Size - HeapEnd + PageSize - 1) div PageSize;
    if _MemGrow(Need) < 0 then
      Exit;
    MemGrown;
    HeapEnd := HeapEnd + Need * PageSize
  end;
  _GetMem := Pointer(HeapPtr);
  HeapPtr := HeapPtr + Size
end;

procedure _FreeMem(P: Pointer; Size: LongInt);
var
  B, Prev, Cur: PFreeBlock;
  A: LongInt;
begin
  Size := _Round8(Size);
  A := LongInt(P);
  B := PFreeBlock(P);
  B^.Size := Size;
  { sortiert einfuegen }
  Prev := nil;
  Cur := FreeList;
  while (Cur <> nil) and (LongInt(Cur) < A) do
  begin
    Prev := Cur;
    Cur := Cur^.Next
  end;
  B^.Next := Cur;
  if Prev = nil then
    FreeList := B
  else
    Prev^.Next := B;
  { mit dem Nachfolger verschmelzen }
  if (Cur <> nil) and (A + B^.Size = LongInt(Cur)) then
  begin
    B^.Size := B^.Size + Cur^.Size;
    B^.Next := Cur^.Next
  end;
  { mit dem Vorgaenger verschmelzen }
  if (Prev <> nil) and (LongInt(Prev) + Prev^.Size = A) then
  begin
    Prev^.Size := Prev^.Size + B^.Size;
    Prev^.Next := B^.Next;
    B := Prev
  end;
  { ein Block am oberen Ende geht an HeapPtr zurueck }
  if LongInt(B) + B^.Size = HeapPtr then
  begin
    HeapPtr := LongInt(B);
    if FreeList = B then
      FreeList := nil
    else
    begin
      Cur := FreeList;
      while Cur^.Next <> B do
        Cur := Cur^.Next;
      Cur^.Next := nil
    end
  end
end;

procedure _Mark(var P: Pointer);
begin
  P := Pointer(HeapPtr)
end;

procedure _Release(P: Pointer);
var
  Prev, Cur: PFreeBlock;
begin
  { TP-ANNAHME: Release verwirft auch freie Bloecke oberhalb von P. }
  HeapPtr := LongInt(P);
  Prev := nil;
  Cur := FreeList;
  while (Cur <> nil) and (LongInt(Cur) < HeapPtr) do
  begin
    Prev := Cur;
    Cur := Cur^.Next
  end;
  if Prev = nil then
    FreeList := nil
  else
    Prev^.Next := nil
end;

function _MemAvail: LongInt;
var
  Cur: PFreeBlock;
  Sum: LongInt;
begin
  Sum := HeapLimit - HeapPtr;
  Cur := FreeList;
  while Cur <> nil do
  begin
    Sum := Sum + Cur^.Size;
    Cur := Cur^.Next
  end;
  _MemAvail := Sum
end;

function _MaxAvail: LongInt;
var
  Cur: PFreeBlock;
  Max: LongInt;
begin
  Max := HeapLimit - HeapPtr;
  Cur := FreeList;
  while Cur <> nil do
  begin
    if Cur^.Size > Max then
      Max := Cur^.Size;
    Cur := Cur^.Next
  end;
  _MaxAvail := Max
end;

{ --------------------------------------------------------------- Strings }

{ Zuweisung mit Kuerzung auf die deklarierte Laenge Max. }
procedure _StrAssign(var Dst: string; const Src: string; Max: Integer);
var
  N: Integer;
begin
  N := Length(Src);
  if N > Max then
    N := Max;
  if N > 0 then
    Move(Src[1], Dst[1], N);
  Dst[0] := Chr(N)
end;

{ Dst := A + B, gekuerzt auf 255 Zeichen. Dst ist nie A oder B. }
procedure _StrConcat(var Dst: string; const A, B: string);
var
  LA, LB: Integer;
begin
  LA := Length(A);
  LB := Length(B);
  if LA + LB > 255 then
    LB := 255 - LA;
  if LA > 0 then
    Move(A[1], Dst[1], LA);
  if LB > 0 then
    Move(B[1], Dst[LA + 1], LB);
  Dst[0] := Chr(LA + LB)
end;

{ Vergleich nach Bytes: -1, 0 oder 1. }
function _StrCmp(const A, B: string): Integer;
var
  I, N: Integer;
begin
  N := Length(A);
  if Length(B) < N then
    N := Length(B);
  for I := 1 to N do
    if A[I] <> B[I] then
    begin
      if A[I] < B[I] then
        _StrCmp := -1
      else
        _StrCmp := 1;
      Exit
    end;
  if Length(A) < Length(B) then
    _StrCmp := -1
  else if Length(A) > Length(B) then
    _StrCmp := 1
  else
    _StrCmp := 0
end;

function _StrCopy(const S: string; Index, Count: Integer): string;
var
  L: Integer;
begin
  L := Length(S);
  { TP-ANNAHME: Ein Index unter 1 zaehlt wie 1. }
  if Index < 1 then
    Index := 1;
  if (Index > L) or (Count <= 0) then
    Count := 0
  else if Count > L - Index + 1 then
    Count := L - Index + 1;
  if Count > 0 then
    Move(S[Index], Result[1], Count);
  Result[0] := Chr(Count)
end;

function _StrPos(const Sub, S: string): Integer;
var
  I, J, LSub: Integer;
  Gleich: Boolean;
begin
  _StrPos := 0;
  LSub := Length(Sub);
  if LSub = 0 then
    Exit;
  for I := 1 to Length(S) - LSub + 1 do
  begin
    Gleich := True;
    J := 1;
    while Gleich and (J <= LSub) do
      if S[I + J - 1] <> Sub[J] then
        Gleich := False
      else
        Inc(J);
    if Gleich then
    begin
      _StrPos := I;
      Exit
    end
  end
end;

procedure _StrDelete(var S: string; Index, Count: Integer);
var
  L: Integer;
begin
  L := Length(S);
  { TP-ANNAHME: Ein Index ausserhalb von 1..Length(S) loescht nichts. }
  if (Index < 1) or (Index > L) or (Count <= 0) then
    Exit;
  if Count > L - Index + 1 then
    Count := L - Index + 1;
  Move(S[Index + Count], S[Index], L - Index - Count + 1);
  S[0] := Chr(L - Count)
end;

{ Fuegt Src vor Position Index in S ein; S hat die deklarierte Laenge Max. }
procedure _StrInsert(const Src: string; var S: string; Index, Max: Integer);
var
  L, LSrc, Tail: Integer;
begin
  L := Length(S);
  LSrc := Length(Src);
  if LSrc = 0 then
    Exit;
  if Index < 1 then
    Index := 1;
  if Index > L then
    Index := L + 1;
  if Index > Max then
    Exit;
  Tail := L - Index + 1;
  if Index - 1 + LSrc > Max then
    LSrc := Max - Index + 1;
  if Index - 1 + LSrc + Tail > Max then
    Tail := Max - Index + 1 - LSrc;
  if Tail > 0 then
    Move(S[Index], S[Index + LSrc], Tail);
  Move(Src[1], S[Index], LSrc);
  S[0] := Chr(Index - 1 + LSrc + Tail)
end;

{ ---------------------------------------------------------------- Mengen }

procedure _SetRange(var S: TSetBytes; Lo, Hi: Integer);
var
  I: Integer;
begin
  if Lo < 0 then
    Lo := 0;
  if Hi > 255 then
    Hi := 255;
  for I := Lo to Hi do
    S[I shr 3] := S[I shr 3] or (1 shl (I and 7))
end;

{ -------------------------------------------------------------- Zeichen }

function _UpCase(C: Char): Char;
begin
  if (C >= 'a') and (C <= 'z') then
    _UpCase := Chr(Ord(C) - 32)
  else
    _UpCase := C
end;

{ ------------------------------------------------------- Zahl in Text }

function _IntStr(V, Width: LongInt): string;
var
  D: LongInt;
  Neg: Boolean;
begin
  Result := '';
  if V = 0 then
    Result := '0';
  Neg := V < 0;
  while V <> 0 do
  begin
    D := V mod 10;
    if D < 0 then
      D := -D;
    Result := Chr(48 + D) + Result;
    V := V div 10
  end;
  if Neg then
    Result := '-' + Result;
  while Length(Result) < Width do
    Result := ' ' + Result
end;

{ Liefert die ersten 17 signifikanten Dezimalziffern von V > 0 und den
  Exponenten mit V = 0.Ziffern * 10^Exp10; fuer V = 0 ist Digits leer. }
procedure RealDigits(V: Real; var Digits: string; var Exp10: LongInt);
var
  M: Real;
  E, I, D: LongInt;
begin
  Digits := '';
  Exp10 := 0;
  if V = 0 then
    Exit;
  M := V;
  E := 0;
  while M >= 1E16 do
  begin
    M := M / 1E16;
    E := E + 16
  end;
  while M >= 10 do
  begin
    M := M / 10;
    Inc(E)
  end;
  while M < 1E-15 do
  begin
    M := M * 1E16;
    E := E - 16
  end;
  while M < 1 do
  begin
    M := M * 10;
    Dec(E)
  end;
  for I := 1 to 17 do
  begin
    D := Trunc(M);
    if D > 9 then
      D := 9;
    Digits := Digits + Chr(48 + D);
    M := (M - D) * 10
  end;
  Exp10 := E + 1
end;

{ Behaelt Keep Ziffern und rundet halbe Werte von null weg (wie TP). }
procedure RoundDigits(var Digits: string; Keep: LongInt; var Exp10: LongInt);
var
  I: LongInt;
  Up: Boolean;
begin
  if Keep < 0 then
  begin
    Digits := '';
    Exit
  end;
  if Keep >= Length(Digits) then
    Exit;
  Up := Digits[Keep + 1] >= '5';
  Digits[0] := Chr(Keep);
  if Up then
  begin
    I := Keep;
    while (I >= 1) and (Digits[I] = '9') do
    begin
      Digits[I] := '0';
      Dec(I)
    end;
    if I < 1 then
    begin
      Digits := '1' + Digits;
      Inc(Exp10)
    end
    else
      Digits[I] := Succ(Digits[I])
  end
end;

{ Formatiert wie Write/Str: mit Prec >= 0 als Festkommazahl, sonst in
  Exponentialschreibweise (Breite 17, wenn Width < 0), rechtsbuendig. }
function _RealStr(V: Real; Width, Prec: LongInt): string;
var
  Neg: Boolean;
  Digits, Frac, T: string;
  Exp10, N, X, I, W: LongInt;
begin
  if V <> V then
    Result := 'NaN'
  else if (V <> 0) and (V * 0 <> 0) then
  begin
    if V > 0 then
      Result := '+Inf'
    else
      Result := '-Inf'
  end
  else
  begin
    Neg := V < 0;
    if Neg then
      V := -V;
    RealDigits(V, Digits, Exp10);
    if Prec >= 0 then
    begin
      RoundDigits(Digits, Exp10 + Prec, Exp10);
      while (Length(Digits) < Exp10 + Prec) and (Length(Digits) < 255) do
        Digits := Digits + '0';
      if Exp10 > 0 then
      begin
        Result := Copy(Digits, 1, Exp10);
        Frac := Copy(Digits, Exp10 + 1, Prec)
      end
      else
      begin
        Result := '0';
        Frac := '';
        for I := 1 to -Exp10 do
          Frac := Frac + '0';
        Frac := Copy(Frac + Digits, 1, Prec)
      end;
      while Length(Frac) < Prec do
        Frac := Frac + '0';
      if Prec > 0 then
        Result := Result + '.' + Frac;
      if Neg then
        for I := 1 to Length(Digits) do
          if Digits[I] <> '0' then
          begin
            Result := '-' + Result;
            Break
          end
    end
    else
    begin
      W := Width;
      if W < 0 then
        W := 17;
      N := W - 7;
      if N < 1 then
        N := 1;
      if Digits = '' then
      begin
        Digits := '0';
        Exp10 := 1
      end;
      RoundDigits(Digits, N + 1, Exp10);
      while Length(Digits) < N + 1 do
        Digits := Digits + '0';
      X := Exp10 - 1;
      if Neg then
        Result := '-'
      else
        Result := ' ';
      Result := Result + Digits[1] + '.' + Copy(Digits, 2, N) + 'E';
      if X < 0 then
      begin
        Result := Result + '-';
        X := -X
      end
      else
        Result := Result + '+';
      T := _IntStr(X, 2);
      if T[1] = ' ' then
        T[1] := '0';
      Result := Result + T
    end
  end;
  while Length(Result) < Width do
    Result := ' ' + Result
end;

{ ------------------------------------------------------- Text in Zahl }

function Digit(C: Char): LongInt;
begin
  case C of
    '0'..'9': Digit := Ord(C) - Ord('0');
    'A'..'F': Digit := Ord(C) - Ord('A') + 10;
    'a'..'f': Digit := Ord(C) - Ord('a') + 10;
  else
    Digit := -1
  end
end;

{ Val fuer Ganzzahlen: dezimal oder $hex, Code = Position des ersten
  ungueltigen Zeichens oder 0. }
function _ValInt(const S: string; var Code: Integer): LongInt;
var
  I, N, D, Base: LongInt;
  R: Real;
  Neg: Boolean;
begin
  _ValInt := 0;
  Code := 0;
  N := Length(S);
  I := 1;
  while (I <= N) and (S[I] = ' ') do
    Inc(I);
  Neg := False;
  if (I <= N) and ((S[I] = '-') or (S[I] = '+')) then
  begin
    Neg := S[I] = '-';
    Inc(I)
  end;
  Base := 10;
  if (I <= N) and (S[I] = '$') then
  begin
    Base := 16;
    Inc(I)
  end;
  if I > N then
  begin
    Code := I;
    Exit
  end;
  R := 0;
  while I <= N do
  begin
    D := Digit(S[I]);
    if (D < 0) or (D >= Base) then
    begin
      Code := I;
      Exit
    end;
    R := R * Base + D;
    if R > 4294967295.0 then
    begin
      Code := I;
      Exit
    end;
    Inc(I)
  end;
  if Base = 16 then
  begin
    if R > 2147483647.0 then
      R := R - 4294967296.0
  end
  else if (R > 2147483648.0) or (not Neg and (R > 2147483647.0)) then
  begin
    Code := N;
    Exit
  end;
  if Neg then
    R := -R;
  _ValInt := Trunc(R)
end;

function Pow10(N: LongInt): Real;
var
  R: Real;
begin
  R := 1;
  while N >= 16 do
  begin
    R := R * 1E16;
    N := N - 16
  end;
  while N > 0 do
  begin
    R := R * 10;
    Dec(N)
  end;
  Pow10 := R
end;

{ Val fuer Real: [Vorzeichen] Ziffern [. Ziffern] [E [Vorzeichen] Ziffern]. }
function _ValReal(const S: string; var Code: Integer): Real;
var
  I, N, Scale, E, D: LongInt;
  M: Real;
  Neg, ENeg: Boolean;
begin
  _ValReal := 0;
  Code := 0;
  N := Length(S);
  I := 1;
  while (I <= N) and (S[I] = ' ') do
    Inc(I);
  Neg := False;
  if (I <= N) and ((S[I] = '-') or (S[I] = '+')) then
  begin
    Neg := S[I] = '-';
    Inc(I)
  end;
  if (I > N) or (Digit(S[I]) < 0) or (Digit(S[I]) > 9) then
  begin
    Code := I;
    Exit
  end;
  M := 0;
  Scale := 0;
  while (I <= N) and (S[I] >= '0') and (S[I] <= '9') do
  begin
    M := M * 10 + (Ord(S[I]) - Ord('0'));
    Inc(I)
  end;
  if (I <= N) and (S[I] = '.') then
  begin
    Inc(I);
    if (I > N) or (S[I] < '0') or (S[I] > '9') then
    begin
      Code := I;
      Exit
    end;
    while (I <= N) and (S[I] >= '0') and (S[I] <= '9') do
    begin
      M := M * 10 + (Ord(S[I]) - Ord('0'));
      Dec(Scale);
      Inc(I)
    end
  end;
  if (I <= N) and ((S[I] = 'E') or (S[I] = 'e')) then
  begin
    Inc(I);
    ENeg := False;
    if (I <= N) and ((S[I] = '-') or (S[I] = '+')) then
    begin
      ENeg := S[I] = '-';
      Inc(I)
    end;
    if (I > N) or (S[I] < '0') or (S[I] > '9') then
    begin
      Code := I;
      Exit
    end;
    E := 0;
    while (I <= N) and (S[I] >= '0') and (S[I] <= '9') do
    begin
      D := Ord(S[I]) - Ord('0');
      if E < 10000 then
        E := E * 10 + D;
      Inc(I)
    end;
    if ENeg then
      E := -E;
    Scale := Scale + E
  end;
  if I <= N then
  begin
    Code := I;
    Exit
  end;
  if Scale > 0 then
    M := M * Pow10(Scale)
  else if Scale < 0 then
  begin
    while Scale < -22 do
    begin
      M := M / 1E22;
      Scale := Scale + 22
    end;
    M := M / Pow10(-Scale)
  end;
  if Neg then
    M := -M;
  _ValReal := M
end;

{ ======================================================= unterste Schicht
  Nur dieser Teil unterscheidet sich zwischen den Zielen. Ergebnisse sind
  WASI-Fehlernummern (0 = Erfolg); Pfade sind schon aufgeloest. }

const
  ENOTCAPABLE = 76;

{$IFNDEF GOJS}   { WASI (auch ohne Zielsymbol) }

function SysWrite(Fd, Buf, Len: LongInt; var Done: LongInt): LongInt;
begin
  IoVec.Buf := Buf;
  IoVec.Len := Len;
  SysWrite := _FdWrite(Fd, LongInt(@IoVec), 1, LongInt(@IoCount));
  Done := IoCount
end;

function SysRead(Fd, Buf, Len: LongInt; var Done: LongInt): LongInt;
begin
  IoVec.Buf := Buf;
  IoVec.Len := Len;
  SysRead := _FdRead(Fd, LongInt(@IoVec), 1, LongInt(@IoCount));
  Done := IoCount
end;

{ Unter WASI liest fd_read blockierend; es liegt nie etwas bereit. }
function InputPending: Boolean;
begin
  InputPending := False
end;

function SysClose(Fd: LongInt): LongInt;
begin
  SysClose := _FdClose(Fd)
end;

function SysSeek(Fd, Offset, Whence: LongInt; var NewOffset: LongInt): LongInt;
var
  R: array[0..1] of LongInt;
begin
  SysSeek := _FdSeek(Fd, Offset, Whence, LongInt(@R));
  NewOffset := R[0]
end;

function SysSize(Fd: LongInt; var Size: LongInt): LongInt;
var
  Stat: array[0..15] of LongInt;
begin
  SysSize := _FdFilestatGet(Fd, LongInt(@Stat));
  Size := Stat[8]   { size liegt bei Offset 32 }
end;

function SysSetSize(Fd, Size: LongInt): LongInt;
begin
  SysSetSize := _FdFilestatSetSize(Fd, Size)
end;

function SysOpen(var P: string; OFlags, Rights, FdFlags: LongInt; var Fd: LongInt): LongInt;
var
  R: LongInt;
begin
  SysOpen := _PathOpen(RootFd, 1, LongInt(@P[1]), Length(P), OFlags, Rights, Rights, FdFlags, LongInt(@R));
  Fd := R
end;

function SysUnlink(var P: string): LongInt;
begin
  SysUnlink := _PathUnlinkFile(RootFd, LongInt(@P[1]), Length(P))
end;

function SysRename(var P, Q: string): LongInt;
begin
  SysRename := _PathRename(RootFd, LongInt(@P[1]), Length(P), RootFd, LongInt(@Q[1]), Length(Q))
end;

function SysMkDir(var P: string): LongInt;
begin
  SysMkDir := _PathCreateDirectory(RootFd, LongInt(@P[1]), Length(P))
end;

function SysRmDir(var P: string): LongInt;
begin
  SysRmDir := _PathRemoveDirectory(RootFd, LongInt(@P[1]), Length(P))
end;

procedure MemGrown;
begin
end;

function _Clock(Id: LongInt): Real;
var
  T: array[0..1] of LongInt;
  Lo: Real;
begin
  _Clock := 0;
  if _ClockTimeGet(Id, 1, LongInt(@T)) = 0 then
  begin
    Lo := T[0];
    if Lo < 0 then
      Lo := Lo + 4294967296.0;
    _Clock := T[1] * 4294967296.0 + Lo
  end
end;

procedure SysArgs;
var
  Sizes: array[0..1] of LongInt;
begin
  ArgStride := 4;
  if _ArgsSizesGet(LongInt(@Sizes[0]), LongInt(@Sizes[1])) <> 0 then
    Exit;
  ArgCount := Sizes[0];
  ArgPtrs := LongInt(_GetMem(ArgCount * 4 + 4));
  ArgBuf := LongInt(_GetMem(Sizes[1] + 1));
  if _ArgsGet(ArgPtrs, ArgBuf) <> 0 then
    ArgCount := 0
end;

{ Fuehrt die Aktion eines Anhaltepunkts aus (der Treiber in _start ruft sie
  zwischen Abwickeln und Fortsetzen). Eingaben liest WASI danach
  blockierend; Delay wartet ueber poll_oneoff. }
function _SuspendAction(Reason, Arg: LongInt): LongInt;
var
  Sub: array[0..11] of LongInt;   { subscription, 48 Byte }
  Ev: array[0..7] of LongInt;     { event, 32 Byte }
  N: LongInt;
  T: Real;
begin
  _SuspendAction := 0;
  if Reason = SuspendHalt then
    _ProcExit(Arg);
  if (Reason = SuspendDelay) and (Arg > 0) then
  begin
    FillChar(Sub, SizeOf(Sub), 0);
    Sub[4] := 1;                      { clock_id: monotonic }
    T := Arg * 1E6;                   { Nanosekunden }
    Sub[7] := Trunc(T / 4294967296.0);
    T := T - Sub[7] * 4294967296.0;
    if T >= 2147483648.0 then
      T := T - 4294967296.0;
    Sub[6] := Trunc(T);
    if _PollOneoff(LongInt(@Sub), LongInt(@Ev), 1, LongInt(@N)) <> 0 then;
  end
end;

{$ENDIF}

{$IFDEF GOJS}

{ Ziel gojs: Schnittstelle von GOOS=js GOARCH=wasm zu wasm_exec.js. Jeder
  Import nimmt den Go-Stackzeiger sp (Adresse von GoSP, Export getsp);
  Argumente und Ergebnisse liegen ab sp+8 in 8-Byte-Faechern. JS-Werte sind
  8-Byte-Referenzen (NaN-Boxing, siehe wasm_exec.js). Eingaben liefert der
  Loader (web/loader.js) zeilenweise: Das Programm ruft
  go.wasmPascalReadLine(f) am Go-Objekt auf; der Loader ruft die
  Callback-Funktion f spaeter (nie innerhalb dieses Aufrufs) mit dem
  naechsten Stueck Eingabe auf, ein leerer String bedeutet Dateiende. }

type
  TRef = record
    Lo, Hi: LongInt
  end;

const
  GoObjRef: TRef = (Lo: 6; Hi: $7FF80001);    { das Go-Objekt aus wasm_exec.js }
  NullRef: TRef = (Lo: 2; Hi: $7FF80000);     { null }

var
  GoSP: array[0..23] of LongInt;
  CallArgs: TRef;
  InputFn: TRef;
  HasInputFn: Boolean;
  PendingTimeout: LongInt;
  WaitingInput: Boolean;
  LineBuf: PChar;                 { Eingabezeile auf dem Heap }
  LineCap, LineLen, LinePos: LongInt;
  NameBuf: string;

function SP: LongInt;
begin
  SP := LongInt(@GoSP)
end;

procedure GoSet(Off, V: LongInt);
begin
  GoSP[Off div 4] := V;
  GoSP[Off div 4 + 1] := 0
end;

procedure GoPutRef(Off: LongInt; const R: TRef);
begin
  GoSP[Off div 4] := R.Lo;
  GoSP[Off div 4 + 1] := R.Hi
end;

function GoGetRef(Off: LongInt): TRef;
begin
  Result.Lo := GoSP[Off div 4];
  Result.Hi := GoSP[Off div 4 + 1]
end;

{ legt einen Go-String (Zeiger, Laenge) ab; der Text liegt in NameBuf }
procedure GoString(Off: LongInt; const S: string);
begin
  NameBuf := S;
  GoSet(Off, LongInt(@NameBuf[1]));
  GoSet(Off + 8, Length(NameBuf))
end;

{ ruft V.Name(A) auf; liefert False, wenn JS eine Ausnahme warf }
function GoCall(const V: TRef; const Name: string; const A: TRef; var R: TRef): Boolean;
begin
  CallArgs := A;
  GoPutRef(8, V);
  GoString(16, Name);
  GoSet(32, LongInt(@CallArgs));
  GoSet(40, 1);
  GoSet(48, 1);
  GoSet(64, 0);
  _GoValueCall(SP);
  R := GoGetRef(56);
  GoCall := (GoSP[16] and 255) <> 0
end;

function GoGet(const V: TRef; const Name: string): TRef;
begin
  GoPutRef(8, V);
  GoString(16, Name);
  _GoValueGet(SP);
  GoGet := GoGetRef(32)
end;

procedure GoFinalize(const R: TRef);
begin
  if (R.Hi and $FFFFFFF8) = $7FF80000 then
  begin
    GoSet(8, R.Lo);
    _GoFinalizeRef(SP)
  end
end;

function SysWrite(Fd, Buf, Len: LongInt; var Done: LongInt): LongInt;
begin
  Done := 0;
  if (Fd <> 1) and (Fd <> 2) then
  begin
    SysWrite := 8;   { EBADF }
    Exit
  end;
  GoSet(8, Fd);
  GoSet(16, Buf);
  GoSet(24, Len);
  _GoWasmWrite(SP);
  Done := Len;
  SysWrite := 0
end;

function SysRead(Fd, Buf, Len: LongInt; var Done: LongInt): LongInt;
var
  N: LongInt;
begin
  Done := 0;
  if Fd <> 0 then
  begin
    SysRead := ENOTCAPABLE;
    Exit
  end;
  N := LineLen - LinePos;
  if N > Len then
    N := Len;
  if N > 0 then
    Move(PChar(Ptr(0, LongInt(LineBuf) + LinePos))^, PChar(Ptr(0, Buf))^, N);
  LinePos := LinePos + N;
  Done := N;
  SysRead := 0
end;

function InputPending: Boolean;
begin
  InputPending := LinePos < LineLen
end;

function SysClose(Fd: LongInt): LongInt;
begin
  SysClose := 0
end;

{ Dateien gibt es im Browser nicht. }
function SysSeek(Fd, Offset, Whence: LongInt; var NewOffset: LongInt): LongInt;
begin
  NewOffset := 0;
  SysSeek := ENOTCAPABLE
end;

function SysSize(Fd: LongInt; var Size: LongInt): LongInt;
begin
  Size := 0;
  SysSize := ENOTCAPABLE
end;

function SysSetSize(Fd, Size: LongInt): LongInt;
begin
  SysSetSize := ENOTCAPABLE
end;

function SysOpen(var P: string; OFlags, Rights, FdFlags: LongInt; var Fd: LongInt): LongInt;
begin
  Fd := -1;
  SysOpen := ENOTCAPABLE
end;

function SysUnlink(var P: string): LongInt;
begin
  SysUnlink := ENOTCAPABLE
end;

function SysRename(var P, Q: string): LongInt;
begin
  SysRename := ENOTCAPABLE
end;

function SysMkDir(var P: string): LongInt;
begin
  SysMkDir := ENOTCAPABLE
end;

function SysRmDir(var P: string): LongInt;
begin
  SysRmDir := ENOTCAPABLE
end;

{ wasm_exec.js haelt eine DataView auf den Speicher, die nach memory.grow
  erneuert werden muss. }
procedure MemGrown;
begin
  _GoResetMemory(SP)
end;

function Int64At(Off: LongInt): Real;
var
  Lo: Real;
begin
  Lo := GoSP[Off div 4];
  if Lo < 0 then
    Lo := Lo + 4294967296.0;
  Int64At := GoSP[Off div 4 + 1] * 4294967296.0 + Lo
end;

function _Clock(Id: LongInt): Real;
begin
  if Id = 1 then
  begin
    _GoNanotime(SP);
    _Clock := Int64At(8)
  end
  else
  begin
    _GoWalltime(SP);
    _Clock := Int64At(8) * 1E9 + GoSP[4]
  end
end;

procedure SysArgs;
begin
  ArgStride := 8;
  ArgCount := _GoArgc;
  ArgPtrs := _GoArgv
end;

procedure ScheduleTimeout(Ms: LongInt);
begin
  GoSet(8, Ms);
  _GoScheduleTimeout(SP);
  PendingTimeout := GoSP[4]   { id bei sp+16 }
end;

{ Fordert beim Loader eine Eingabezeile an. Ohne Loader-Funktion gilt die
  Eingabe als beendet, und das Programm wird sofort fortgesetzt. }
procedure RequestInput;
var
  R, Num: TRef;
begin
  if not HasInputFn then
  begin
    Num.Lo := 0;
    Num.Hi := $3FF00000;   { die Zahl 1.0 als Kennung der Callback-Funktion }
    if GoCall(GoObjRef, '_makeFuncWrapper', Num, InputFn) then
      HasInputFn := True
  end;
  if HasInputFn and GoCall(GoObjRef, 'wasmPascalReadLine', InputFn, R) then
    WaitingInput := True
  else
  begin
    LineLen := 0;
    LinePos := 0;
    ScheduleTimeout(0)
  end
end;

{ Liest die Zeile aus go._pendingEvent.args[0] in den Zeilenpuffer. }
procedure ReadPendingEvent;
var
  Ev, Args, Str, Bytes: TRef;
  N: LongInt;
begin
  Ev := GoGet(GoObjRef, '_pendingEvent');
  Args := GoGet(Ev, 'args');
  GoPutRef(8, Args);
  GoSet(16, 0);
  _GoValueIndex(SP);
  Str := GoGetRef(24);
  GoPutRef(8, Str);
  _GoValuePrepareString(SP);
  Bytes := GoGetRef(16);
  N := GoSP[6];
  if N > LineCap then
  begin
    if LineCap > 0 then
      _FreeMem(LineBuf, LineCap);
    LineCap := N;
    LineBuf := _GetMem(LineCap)
  end;
  GoPutRef(8, Bytes);
  GoSet(16, LongInt(LineBuf));
  GoSet(24, N);
  GoSet(32, N);
  if N > 0 then
    _GoValueLoadString(SP);
  LineLen := N;
  LinePos := 0;
  { go._pendingEvent := null }
  GoPutRef(8, GoObjRef);
  GoString(16, '_pendingEvent');
  GoPutRef(32, NullRef);
  _GoValueSet(SP);
  GoFinalize(Ev);
  GoFinalize(Args);
  GoFinalize(Str);
  GoFinalize(Bytes)
end;

{ Laeuft vor dem Fortsetzen (Export resume): Timer abmelden, damit
  wasm_exec.js ihn nicht erneut ausloest, und eine gelieferte Zeile
  uebernehmen. }
function _GoResumeAction: LongInt;
begin
  if PendingTimeout <> 0 then
  begin
    GoSet(8, PendingTimeout);
    _GoClearTimeout(SP);
    PendingTimeout := 0
  end;
  if WaitingInput then
  begin
    WaitingInput := False;
    ReadPendingEvent
  end;
  _GoResumeAction := 0
end;

{ Aktion eines Anhaltepunkts: Timer setzen, Eingabe anfordern oder beenden.
  Danach kehren run bzw. resume zu wasm_exec.js zurueck. }
function _SuspendAction(Reason, Arg: LongInt): LongInt;
begin
  _SuspendAction := 0;
  case Reason of
    SuspendDelay: ScheduleTimeout(Arg);
    SuspendYield: ScheduleTimeout(0);
    SuspendInput: RequestInput;
    SuspendHalt:
      begin
        GoSet(8, Arg);
        _GoWasmExit(SP)
      end;
  end
end;

{$ENDIF}

{ ------------------------------------------------------------- Dateien }

function IoError(Errno, Default: LongInt): Integer;
begin
  case Errno of
    44: IoError := 2;                 { ENOENT: Datei nicht gefunden }
    54: IoError := 3;                 { ENOTDIR: Pfad nicht gefunden }
    33: IoError := 4;                 { EMFILE: zu viele offene Dateien }
    2, 20, 31, 55, 63, 76: IoError := 5; { Zugriff verweigert }
  else
    IoError := Default
  end
end;

function IsAssigned(Mode: LongInt): Boolean;
begin
  IsAssigned := (Mode >= fmClosed) and (Mode <= fmInOut)
end;

{ Prueft den Modus fuer eine Operation; setzt sonst InOutRes. }
function CheckMode(Mode, Want: LongInt): Boolean;
begin
  CheckMode := False;
  if InOutRes <> 0 then
    Exit;
  if (Mode = Want) or ((Want = fmInOut) and (Mode >= fmInput)) then
    CheckMode := True
  else if Mode = fmClosed then
    InOutRes := 103
  else if not IsAssigned(Mode) then
    InOutRes := 102
  else if Want = fmInput then
    InOutRes := 104
  else
    InOutRes := 105
end;

{ Pascal-Dateiname in einen Pfad relativ zum Wurzelverzeichnis. }
function _Path(const Name: string): string;
var
  I: Integer;
begin
  Result := Name;
  for I := 1 to Length(Result) do
    if Result[I] = '\' then
      Result[I] := '/';
  if (Length(Result) > 0) and (Result[1] = '/') then
    while (Length(Result) > 0) and (Result[1] = '/') do
      Delete(Result, 1, 1)
  else if CurDir <> '' then
    Result := CurDir + '/' + Result;
  if Result = '' then
    Result := '.'
end;

function OpenPath(const Name: string; OFlags, Rights, FdFlags: LongInt; var Handle: LongInt): Integer;
var
  P: string;
  Fd, Err: LongInt;
begin
  P := _Path(Name);
  Err := SysOpen(P, OFlags, Rights, FdFlags, Fd);
  if Err = 0 then
  begin
    Handle := Fd;
    OpenPath := 0
  end
  else
    OpenPath := IoError(Err, 5)
end;

function WriteBytes(Handle, Buf, Len: LongInt): Boolean;
var
  Done: LongInt;
begin
  WriteBytes := True;
  while Len > 0 do
  begin
    if (SysWrite(Handle, Buf, Len, Done) <> 0) or (Done <= 0) then
    begin
      WriteBytes := False;
      Exit
    end;
    Buf := Buf + Done;
    Len := Len - Done
  end
end;

function ReadBytes(Handle, Buf, Len: LongInt): LongInt;
var
  Got, Done: LongInt;
begin
  Got := 0;
  while Got < Len do
  begin
    if SysRead(Handle, Buf + Got, Len - Got, Done) <> 0 then
    begin
      ReadBytes := -1;
      Exit
    end;
    if Done = 0 then
      Break;
    Got := Got + Done
  end;
  ReadBytes := Got
end;

function _IOResult: Integer;
begin
  _IOResult := InOutRes;
  InOutRes := 0
end;

{ ------------------------------------------------------------ Scheduler }

{ Monotone Uhr in Millisekunden. }
function NowMs: Real;
begin
  NowMs := _Clock(1) / 1E6
end;

procedure FlushText(var F: TTextRec); forward;

{ Haelt an: Ausgaben leeren, Kontrolle an den Host, danach weiter. }
procedure Suspend(Reason, Arg: LongInt);
begin
  FlushText(TTextRec(Output));
  if _Suspend(Reason, Arg) <> 0 then;
  LastYield := NowMs
end;

procedure _Assign(var F: TFileRec; const Name: string);
begin
  F.Handle := -1;
  F.Mode := fmClosed;
  F.RecSize := 0;
  F.Reserved := 0;
  F.Name := Name
end;

{ --------------------------------------------------------- Textdateien }

procedure FlushText(var F: TTextRec);
begin
  if (F.Mode = fmOutput) and (F.BufPos > 0) then
  begin
    if not WriteBytes(F.Handle, LongInt(@F.Buffer), F.BufPos) and (InOutRes = 0) then
      InOutRes := 101;
    F.BufPos := 0
  end
end;

procedure _Flush(var F: TTextRec);
begin
  if CheckMode(F.Mode, fmOutput) then
    FlushText(F)
end;

procedure CloseText(var F: TTextRec);
begin
  FlushText(F);
  if F.Handle > 2 then
    if SysClose(F.Handle) <> 0 then;
  F.Mode := fmClosed
end;

procedure OpenText(var F: TTextRec; Mode, OFlags, Rights, FdFlags, StdHandle: LongInt);
var
  Err: Integer;
begin
  if InOutRes <> 0 then
    Exit;
  if not IsAssigned(F.Mode) then
  begin
    InOutRes := 102;
    Exit
  end;
  if F.Mode <> fmClosed then
    CloseText(F);
  if F.Name = '' then
    F.Handle := StdHandle
  else
  begin
    Err := OpenPath(F.Name, OFlags, Rights, FdFlags, F.Handle);
    if Err <> 0 then
    begin
      InOutRes := Err;
      Exit
    end
  end;
  F.Mode := Mode;
  F.BufPos := 0;
  F.BufEnd := 0
end;

procedure _ResetText(var F: TTextRec);
begin
  OpenText(F, fmInput, 0, RightsRead, 0, 0)
end;

procedure _RewriteText(var F: TTextRec);
begin
  OpenText(F, fmOutput, 9, RightsWrite, 0, 1)   { O_CREAT + O_TRUNC }
end;

procedure _AppendText(var F: TTextRec);
begin
  OpenText(F, fmOutput, 0, RightsWrite, 1, 1)   { FDFLAGS_APPEND }
end;

procedure _CloseText(var F: TTextRec);
begin
  if CheckMode(F.Mode, fmInOut) then
    CloseText(F)
end;

procedure PutChar(var F: TTextRec; C: Char);
begin
  if F.BufPos >= BufSize then
    FlushText(F);
  F.Buffer[F.BufPos] := C;
  Inc(F.BufPos)
end;

procedure _WriteStr(var F: TTextRec; const S: string; Width: LongInt);
var
  I: LongInt;
begin
  if not CheckMode(F.Mode, fmOutput) then
    Exit;
  for I := Length(S) + 1 to Width do
    PutChar(F, ' ');
  for I := 1 to Length(S) do
    PutChar(F, S[I])
end;

procedure _WriteChar(var F: TTextRec; C: Char; Width: LongInt);
var
  I: LongInt;
begin
  if not CheckMode(F.Mode, fmOutput) then
    Exit;
  for I := 2 to Width do
    PutChar(F, ' ');
  PutChar(F, C)
end;

procedure _WriteInt(var F: TTextRec; V, Width: LongInt);
begin
  _WriteStr(F, _IntStr(V, Width), 0)
end;

procedure _WriteReal(var F: TTextRec; V: Real; Width, Prec: LongInt);
begin
  _WriteStr(F, _RealStr(V, Width, Prec), 0)
end;

procedure _WriteBool(var F: TTextRec; B: Boolean; Width: LongInt);
begin
  if B then
    _WriteStr(F, 'TRUE', Width)
  else
    _WriteStr(F, 'FALSE', Width)
end;

procedure _WriteLn(var F: TTextRec);
begin
  if CheckMode(F.Mode, fmOutput) then
    PutChar(F, #10)
end;

{ Nach jedem Write/WriteLn: Konsolenausgaben sofort weitergeben; nach mehr
  als 30 ms seit dem letzten Anhalten ist Write ein Anhaltepunkt. }
procedure _WriteEnd(var F: TTextRec);
begin
  if (F.Mode = fmOutput) and (F.Handle <= 2) then
  begin
    FlushText(F);
    if NowMs - LastYield > YieldMs then
      Suspend(SuspendYield, 0)
  end
end;

{ Liefert False am Dateiende; liest bei Bedarf nach. }
function PeekChar(var F: TTextRec; var C: Char): Boolean;
var
  N: LongInt;
begin
  if F.BufPos >= F.BufEnd then
  begin
    if (F.Handle = 0) and not InputPending then
      Suspend(SuspendInput, 0);   { Anhaltepunkt: Eingabe gebraucht }
    if SysRead(F.Handle, LongInt(@F.Buffer), BufSize, N) <> 0 then
    begin
      if InOutRes = 0 then
        InOutRes := 100;
      N := 0
    end;
    F.BufPos := 0;
    F.BufEnd := N
  end;
  if F.BufPos < F.BufEnd then
  begin
    C := F.Buffer[F.BufPos];
    PeekChar := True
  end
  else
  begin
    C := #26;
    PeekChar := False
  end
end;

function _ReadChar(var F: TTextRec): Char;
var
  C: Char;
begin
  _ReadChar := #26;
  if not CheckMode(F.Mode, fmInput) then
    Exit;
  if PeekChar(F, C) then
  begin
    _ReadChar := C;
    Inc(F.BufPos)
  end
end;

procedure _ReadStr(var F: TTextRec; var S: string; Max: LongInt);
var
  C: Char;
  N: LongInt;
begin
  S[0] := #0;
  if not CheckMode(F.Mode, fmInput) then
    Exit;
  N := 0;
  while (N < Max) and PeekChar(F, C) and (C <> #10) and (C <> #13) do
  begin
    Inc(N);
    S[N] := C;
    Inc(F.BufPos)
  end;
  S[0] := Chr(N)
end;

procedure ReadToken(var F: TTextRec; var T: string);
var
  C: Char;
begin
  T := '';
  while PeekChar(F, C) and ((C = ' ') or (C = #9) or (C = #10) or (C = #13)) do
    Inc(F.BufPos);
  while PeekChar(F, C) and (C <> ' ') and (C <> #9) and (C <> #10) and (C <> #13) and (Length(T) < 255) do
  begin
    T := T + C;
    Inc(F.BufPos)
  end
end;

function _ReadInt(var F: TTextRec): LongInt;
var
  T: string;
  Code: Integer;
  V: LongInt;
begin
  _ReadInt := 0;
  if not CheckMode(F.Mode, fmInput) then
    Exit;
  ReadToken(F, T);
  if T = '' then
    Exit;
  V := _ValInt(T, Code);
  if Code <> 0 then
    InOutRes := 106
  else
    _ReadInt := V
end;

function _ReadReal(var F: TTextRec): Real;
var
  T: string;
  Code: Integer;
  V: Real;
begin
  _ReadReal := 0;
  if not CheckMode(F.Mode, fmInput) then
    Exit;
  ReadToken(F, T);
  if T = '' then
    Exit;
  V := _ValReal(T, Code);
  if Code <> 0 then
    InOutRes := 106
  else
    _ReadReal := V
end;

procedure _ReadLn(var F: TTextRec);
var
  C: Char;
begin
  if not CheckMode(F.Mode, fmInput) then
    Exit;
  while PeekChar(F, C) and (C <> #10) do
    Inc(F.BufPos);
  if PeekChar(F, C) then
    Inc(F.BufPos)
end;

function _EofText(var F: TTextRec): Boolean;
var
  C: Char;
begin
  _EofText := True;
  if CheckMode(F.Mode, fmInput) then
    _EofText := not PeekChar(F, C)
end;

function _Eoln(var F: TTextRec): Boolean;
var
  C: Char;
begin
  _Eoln := True;
  if CheckMode(F.Mode, fmInput) then
    _Eoln := not PeekChar(F, C) or (C = #10) or (C = #13)
end;

function _SeekEof(var F: TTextRec): Boolean;
var
  C: Char;
begin
  _SeekEof := True;
  if not CheckMode(F.Mode, fmInput) then
    Exit;
  while PeekChar(F, C) and ((C = ' ') or (C = #9) or (C = #10) or (C = #13)) do
    Inc(F.BufPos);
  _SeekEof := not PeekChar(F, C)
end;

function _SeekEoln(var F: TTextRec): Boolean;
var
  C: Char;
begin
  _SeekEoln := True;
  if not CheckMode(F.Mode, fmInput) then
    Exit;
  while PeekChar(F, C) and ((C = ' ') or (C = #9)) do
    Inc(F.BufPos);
  _SeekEoln := not PeekChar(F, C) or (C = #10) or (C = #13)
end;

{ ------------------------------------- typisierte und untypisierte Dateien }

procedure OpenFile(var F: TFileRec; RecSize, OFlags: LongInt);
var
  Err: Integer;
begin
  if InOutRes <> 0 then
    Exit;
  if not IsAssigned(F.Mode) then
  begin
    InOutRes := 102;
    Exit
  end;
  if (F.Mode <> fmClosed) and (F.Handle > 2) then
    if SysClose(F.Handle) <> 0 then;
  Err := OpenPath(F.Name, OFlags, RightsWrite, 0, F.Handle);
  if Err <> 0 then
  begin
    F.Mode := fmClosed;
    InOutRes := Err;
    Exit
  end;
  F.Mode := fmInOut;
  F.RecSize := RecSize
end;

procedure _ResetFile(var F: TFileRec; RecSize: LongInt);
begin
  OpenFile(F, RecSize, 0)
end;

procedure _RewriteFile(var F: TFileRec; RecSize: LongInt);
begin
  OpenFile(F, RecSize, 9)
end;

procedure _CloseFile(var F: TFileRec);
begin
  if CheckMode(F.Mode, fmInOut) then
  begin
    if F.Handle > 2 then
      if SysClose(F.Handle) <> 0 then;
    F.Mode := fmClosed
  end
end;

procedure _ReadRec(var F: TFileRec; Buf: LongInt);
begin
  if CheckMode(F.Mode, fmInOut) then
    if ReadBytes(F.Handle, Buf, F.RecSize) <> F.RecSize then
      InOutRes := 100
end;

procedure _WriteRec(var F: TFileRec; Buf: LongInt);
begin
  if CheckMode(F.Mode, fmInOut) then
    if not WriteBytes(F.Handle, Buf, F.RecSize) then
      InOutRes := 101
end;

function _BlockRead(var F: TFileRec; Buf, Count: LongInt; WantResult: Boolean): LongInt;
var
  Got: LongInt;
begin
  _BlockRead := 0;
  if not CheckMode(F.Mode, fmInOut) then
    Exit;
  Got := ReadBytes(F.Handle, Buf, Count * F.RecSize);
  if Got < 0 then
  begin
    InOutRes := 100;
    Exit
  end;
  _BlockRead := Got div F.RecSize;
  if not WantResult and (Got < Count * F.RecSize) then
    InOutRes := 100
end;

function _BlockWrite(var F: TFileRec; Buf, Count: LongInt; WantResult: Boolean): LongInt;
begin
  _BlockWrite := 0;
  if not CheckMode(F.Mode, fmInOut) then
    Exit;
  if WriteBytes(F.Handle, Buf, Count * F.RecSize) then
    _BlockWrite := Count
  else if not WantResult then
    InOutRes := 101
end;

function SeekTo(Handle, Offset, Whence: LongInt): LongInt;
var
  NewOffset: LongInt;
begin
  if SysSeek(Handle, Offset, Whence, NewOffset) <> 0 then
  begin
    if InOutRes = 0 then
      InOutRes := 100;
    SeekTo := 0
  end
  else
    SeekTo := NewOffset
end;

procedure _Seek(var F: TFileRec; N: LongInt);
begin
  if CheckMode(F.Mode, fmInOut) then
    if SeekTo(F.Handle, N * F.RecSize, 0) = 0 then;
end;

function _FilePos(var F: TFileRec): LongInt;
begin
  _FilePos := 0;
  if CheckMode(F.Mode, fmInOut) then
    _FilePos := SeekTo(F.Handle, 0, 1) div F.RecSize
end;

function _FileSize(var F: TFileRec): LongInt;
var
  Size: LongInt;
begin
  _FileSize := 0;
  if not CheckMode(F.Mode, fmInOut) then
    Exit;
  if SysSize(F.Handle, Size) <> 0 then
    InOutRes := 100
  else
    _FileSize := Size div F.RecSize
end;

function _EofFile(var F: TFileRec): Boolean;
begin
  _EofFile := _FilePos(F) >= _FileSize(F)
end;

procedure _Truncate(var F: TFileRec);
begin
  if CheckMode(F.Mode, fmInOut) then
    if SysSetSize(F.Handle, SeekTo(F.Handle, 0, 1)) <> 0 then
      InOutRes := 101
end;

procedure _Erase(var F: TFileRec);
var
  P: string;
  Err: LongInt;
begin
  if InOutRes <> 0 then
    Exit;
  if not IsAssigned(F.Mode) then
  begin
    InOutRes := 102;
    Exit
  end;
  P := _Path(F.Name);
  Err := SysUnlink(P);
  if Err <> 0 then
    InOutRes := IoError(Err, 5)
end;

procedure _Rename(var F: TFileRec; const NewName: string);
var
  P, Q: string;
  Err: LongInt;
begin
  if InOutRes <> 0 then
    Exit;
  if not IsAssigned(F.Mode) then
  begin
    InOutRes := 102;
    Exit
  end;
  P := _Path(F.Name);
  Q := _Path(NewName);
  Err := SysRename(P, Q);
  if Err <> 0 then
    InOutRes := IoError(Err, 5)
  else
    F.Name := NewName
end;

{ ---------------------------------------------------------- Verzeichnisse }

procedure _ChDir(const Dir: string);
var
  P: string;
  Fd: LongInt;
begin
  if InOutRes <> 0 then
    Exit;
  P := _Path(Dir);
  if OpenPath(Dir, 2, 0, 0, Fd) <> 0 then   { O_DIRECTORY, ohne Rechte }
  begin
    InOutRes := 3;
    Exit
  end;
  if SysClose(Fd) <> 0 then;
  while (Length(P) > 0) and (P[Length(P)] = '/') do
    Delete(P, Length(P), 1);
  if P = '.' then
    P := '';
  CurDir := P
end;

procedure _MkDir(const Dir: string);
var
  P: string;
  Err: LongInt;
begin
  if InOutRes <> 0 then
    Exit;
  P := _Path(Dir);
  Err := SysMkDir(P);
  if Err <> 0 then
    InOutRes := IoError(Err, 5)
end;

procedure _RmDir(const Dir: string);
var
  P: string;
  Err: LongInt;
begin
  if InOutRes <> 0 then
    Exit;
  P := _Path(Dir);
  Err := SysRmDir(P);
  if Err <> 0 then
    InOutRes := IoError(Err, 5)
end;

function _GetDir(Drive: LongInt): string;
begin
  _GetDir := '/' + CurDir
end;

{ ----------------------------------------------------------- Mathematik }

const
  Ln2Hi = 6.93147180369123816490E-01;
  Ln2Lo = 1.90821492927058770002E-10;
  PiHalfHi = 1.57079632679489655800E+00;
  PiHalfLo = 6.12323399573676588613E-17;

function Scale2(X: Real; N: LongInt): Real;
begin
  while N > 0 do
  begin
    X := X * 2;
    Dec(N)
  end;
  while N < 0 do
  begin
    X := X / 2;
    Inc(N)
  end;
  Scale2 := X
end;

function _Exp(X: Real): Real;
var
  N, I: LongInt;
  R, Term, Sum: Real;
begin
  if X > 709.8 then
  begin
    _Exp := 1E308 * 10;   { +Inf }
    Exit
  end;
  if X < -745.2 then
  begin
    _Exp := 0;
    Exit
  end;
  if X >= 0 then
    N := Trunc(X / 0.6931471805599453 + 0.5)
  else
    N := Trunc(X / 0.6931471805599453 - 0.5);
  R := (X - N * Ln2Hi) - N * Ln2Lo;
  Sum := 1;
  Term := 1;
  for I := 1 to 20 do
  begin
    Term := Term * R / I;
    Sum := Sum + Term
  end;
  _Exp := Scale2(Sum, N)
end;

function _Ln(X: Real): Real;
var
  K, I: LongInt;
  M, S, S2, Term, Sum: Real;
begin
  if X <= 0 then
    _RunError(207);
  if X * 0 <> 0 then
  begin
    _Ln := X;
    Exit
  end;
  K := 0;
  M := X;
  while M >= 2 do
  begin
    M := M / 2;
    Inc(K)
  end;
  while M < 1 do
  begin
    M := M * 2;
    Dec(K)
  end;
  if M > 1.4142135623730951 then
  begin
    M := M / 2;
    Inc(K)
  end;
  S := (M - 1) / (M + 1);
  S2 := S * S;
  Sum := 0;
  Term := S;
  for I := 0 to 30 do
  begin
    Sum := Sum + Term / (2 * I + 1);
    Term := Term * S2
  end;
  _Ln := 2 * Sum + (K * Ln2Lo + K * Ln2Hi)
end;

{ Reduziert X auf R in [-Pi/4, Pi/4] mit X = R + Q * Pi/2. }
function Reduce(X: Real; var Q: LongInt): Real;
var
  N: Real;
begin
  if X >= 0 then
    N := Int(X / PiHalfHi + 0.5)
  else
    N := Int(X / PiHalfHi - 0.5);
  Q := Trunc(N - 4 * Int(N / 4));
  if Q < 0 then
    Q := Q + 4;
  Reduce := (X - N * PiHalfHi) - N * PiHalfLo
end;

function SinPoly(R: Real): Real;
var
  I: LongInt;
  Term, Sum, R2: Real;
begin
  R2 := R * R;
  Term := R;
  Sum := R;
  for I := 1 to 12 do
  begin
    Term := -Term * R2 / ((2 * I) * (2 * I + 1));
    Sum := Sum + Term
  end;
  SinPoly := Sum
end;

function CosPoly(R: Real): Real;
var
  I: LongInt;
  Term, Sum, R2: Real;
begin
  R2 := R * R;
  Term := 1;
  Sum := 1;
  for I := 1 to 12 do
  begin
    Term := -Term * R2 / ((2 * I - 1) * (2 * I));
    Sum := Sum + Term
  end;
  CosPoly := Sum
end;

function _Sin(X: Real): Real;
var
  Q: LongInt;
  R: Real;
begin
  R := Reduce(X, Q);
  case Q of
    0: _Sin := SinPoly(R);
    1: _Sin := CosPoly(R);
    2: _Sin := -SinPoly(R);
  else
    _Sin := -CosPoly(R)
  end
end;

function _Cos(X: Real): Real;
var
  Q: LongInt;
  R: Real;
begin
  R := Reduce(X, Q);
  case Q of
    0: _Cos := CosPoly(R);
    1: _Cos := -SinPoly(R);
    2: _Cos := -CosPoly(R);
  else
    _Cos := SinPoly(R)
  end
end;

function _ArcTan(X: Real): Real;
var
  Neg, Inv: Boolean;
  Add, Term, Sum, X2: Real;
  I: LongInt;
begin
  Neg := X < 0;
  if Neg then
    X := -X;
  Inv := X > 1;
  if Inv then
    X := 1 / X;
  Add := 0;
  if X > 0.2679491924311227 then   { tan(Pi/12) }
  begin
    X := (X * 1.7320508075688772 - 1) / (X + 1.7320508075688772);
    Add := 0.5235987755982989      { Pi/6 }
  end;
  X2 := X * X;
  Term := X;
  Sum := 0;
  for I := 0 to 40 do
  begin
    Sum := Sum + Term / (2 * I + 1);
    Term := -Term * X2
  end;
  Sum := Sum + Add;
  if Inv then
    Sum := 1.5707963267948966 - Sum;
  if Neg then
    Sum := -Sum;
  _ArcTan := Sum
end;

{ --------------------------------------------------------------- Zufall }

{ Lineare Kongruenz wie in TP: RandSeed := RandSeed * 134775813 + 1. }
function NextRandom: Real;
var
  U: Real;
begin
  RandSeed := RandSeed * 134775813 + 1;
  U := RandSeed;
  if U < 0 then
    U := U + 4294967296.0;
  NextRandom := U / 4294967296.0
end;

function _Random(Range: LongInt): LongInt;
begin
  if Range <= 0 then
    _Random := 0
  else
    _Random := Trunc(NextRandom * Range)
end;

function _RandomReal: Real;
begin
  _RandomReal := NextRandom
end;

procedure _Randomize;
var
  T: Real;
begin
  T := _Clock(0);
  T := T - Int(T / 4294967296.0) * 4294967296.0;
  RandSeed := Trunc(T - 2147483648.0)
end;

{ ------------------------------------------------------------ Parameter }

procedure LoadArgs;
begin
  if ArgStride <> 0 then
    Exit;
  SysArgs
end;

function _ParamCount: Integer;
begin
  LoadArgs;
  if ArgCount > 0 then
    _ParamCount := ArgCount - 1
  else
    _ParamCount := 0
end;

function _ParamStr(I: LongInt): string;
var
  P, N: LongInt;
begin
  LoadArgs;
  Result := '';
  if (I < 0) or (I >= ArgCount) then
    Exit;
  P := PLongInt(Ptr(0, ArgPtrs + I * ArgStride))^;
  N := 0;
  while (N < 255) and (PChar(Ptr(0, P + N))^ <> #0) do
  begin
    Result[N + 1] := PChar(Ptr(0, P + N))^;
    Inc(N)
  end;
  Result[0] := Chr(N)
end;

{ ---------------------------------------------------------- Programmende }

procedure _Done;
begin
  FlushText(TTextRec(Output))
end;

{ Halt ist ein Anhaltepunkt: Die Laufzeit beendet das Programm in der
  Aktion (WASI: proc_exit, gojs: wasmExit). }
procedure _Halt(Code: LongInt);
begin
  _Done;
  Suspend(SuspendHalt, Code)
end;

procedure InitText(var F: TTextRec; Handle, Mode: LongInt);
begin
  F.Handle := Handle;
  F.Mode := Mode;
  F.BufPos := 0;
  F.BufEnd := 0;
  F.Name := ''
end;

begin
  HeapPtr := _HeapBase;
  HeapEnd := _MemSize * PageSize;
  FreeList := nil;
  InOutRes := 0;
  CurDir := '';
  ArgPtrs := 0;
  ArgStride := 0;
  {$IFDEF GOJS}
  _GoSetSP(SP);
  HasInputFn := False;
  PendingTimeout := 0;
  WaitingInput := False;
  LineCap := 0;
  LineLen := 0;
  LinePos := 0;
  {$ENDIF}
  RandSeed := 0;
  FileMode := 2;
  ExitCode := 0;
  LastYield := NowMs;
  InitText(TTextRec(Input), 0, fmInput);
  InitText(TTextRec(Output), 1, fmOutput)
end.
