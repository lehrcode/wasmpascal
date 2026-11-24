{ System-Unit von WasmPascal.

  Sie wird implizit vor jedem Programm uebersetzt. Routinen, deren Name mit
  einem Unterstrich beginnt, ruft der Compiler fuer Sprachmittel auf (Heap,
  Strings, Mengen); Anwenderprogramme benutzen sie nicht direkt.
  Etappe 6 ergaenzt hier die uebrige Laufzeit (E/A, Formatierung, Mathematik).

  Intrinsics des Compilers: _HeapBase (erste Adresse hinter dem
  Schatten-Stack), _MemSize (Speichergroesse in Seiten), _MemGrow(n)
  (memory.grow, liefert -1 bei Fehlschlag). }
unit System;

interface

implementation

type
  PFreeBlock = ^TFreeBlock;
  TFreeBlock = record
    Next: PFreeBlock;
    Size: LongInt
  end;
  TSetBytes = array[0..31] of Byte;

const
  { Obergrenze des Heaps, damit Adressen als LongInt positiv bleiben. }
  HeapLimit = $7FFF0000;
  PageSize = 65536;

var
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

begin
  HeapPtr := _HeapBase;
  HeapEnd := _MemSize * PageSize;
  FreeList := nil
end.
