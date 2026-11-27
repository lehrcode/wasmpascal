{ Prozedurale Typen wie in Turbo Pascal 5.0 }
program ProzVar;

type
  TProc = procedure;
  TIntProc = procedure(X: Integer);
  TFunc = function(X: Real): Real;
  TBinOp = function(A, B: LongInt): LongInt;
  TZaehler = function: Integer;
  TVarProc = procedure(var S: string; const T: string);
  TStrFunc = function(N: Integer): string;
  TOps = array[1..3] of TBinOp;
  TEintrag = record
    Name: string[10];
    Op: TBinOp
  end;

var
  P: TProc;
  Q: TIntProc;
  F, G: TFunc;
  Op: TBinOp;
  Z, Z2: TZaehler;
  V: TVarProc;
  SF: TStrFunc;
  Ops: TOps;
  E: TEintrag;
  Ptr: Pointer;
  S: string;
  I, Stand: Integer;

procedure Hallo;
begin
  WriteLn('Hallo')
end;

procedure Zeige(X: Integer);
begin
  WriteLn('Zeige ', X)
end;

function Quadrat(X: Real): Real;
begin
  Quadrat := X * X
end;

function Halb(X: Real): Real;
begin
  Halb := X / 2
end;

function Plus(A, B: LongInt): LongInt;
begin
  Plus := A + B
end;

function Minus(A, B: LongInt): LongInt;
begin
  Minus := A - B
end;

function Mal(A, B: LongInt): LongInt;
begin
  Mal := A * B
end;

function Naechster: Integer;
begin
  Inc(Stand);
  Naechster := Stand
end;

procedure Anhaengen(var S: string; const T: string);
begin
  S := S + T
end;

function Sterne(N: Integer): string;
var
  K: Integer;
  T: string;
begin
  T := '';
  for K := 1 to N do
    T := T + '*';
  Sterne := T
end;

{ Funktionsparameter }
function Tabelle(F: TFunc; A, B: Real): Real;
var
  Summe: Real;
  X: Real;
begin
  Summe := 0;
  X := A;
  while X <= B do
  begin
    Summe := Summe + F(X);
    X := X + 1
  end;
  Tabelle := Summe
end;

procedure Setze(var P: TProc);
begin
  P := Hallo
end;

const
  KOps: TOps = (Plus, Minus, Mal);
  KEintrag: TEintrag = (Name: 'mal'; Op: Mal);
  KLeer: TProc = nil;

begin
  P := Hallo;
  P;
  Q := Zeige;
  Q(42);
  F := Quadrat;
  WriteLn(F(3):0:1);
  G := F;                        { Kopie, kein Aufruf }
  WriteLn(G(4):0:1);
  F := Halb;
  WriteLn(F(5):0:1, ' ', G(5):0:1);
  WriteLn(Tabelle(Quadrat, 1, 3):0:1);
  WriteLn(Tabelle(Halb, 1, 3):0:1);

  { parameterlose Funktionsvariable: im Ausdruck ein Aufruf }
  Stand := 10;
  Z := Naechster;
  Z2 := Z;
  I := Z + Z2;
  WriteLn('Z: ', I, ' ', Stand);

  Ops[1] := Plus;
  Ops[2] := Minus;
  Ops[3] := Mal;
  for I := 1 to 3 do
    Write(Ops[I](7, 3), ' ');
  WriteLn;
  for I := 1 to 3 do
    Write(KOps[I](10, 2), ' ');
  WriteLn;
  E.Name := 'plus';
  E.Op := Plus;
  WriteLn(E.Name, ' ', E.Op(2, 3));
  with KEintrag do
    WriteLn(Name, ' ', Op(2, 3));

  S := 'ab';
  V := Anhaengen;
  V(S, 'cd');
  WriteLn(S);
  SF := Sterne;
  WriteLn(SF(5), '|', Length(SF(3)));

  { nil, @ und Typumwandlung }
  P := nil;
  WriteLn(@P = nil, ' ', @KLeer = nil);
  Setze(P);
  WriteLn(@P <> nil);
  P;
  Ptr := @Zeige;
  Q := nil;
  Q := TIntProc(Ptr);
  Q(7);
  WriteLn(@Q = @Zeige, ' ', @Q = Ptr, ' ', @Q = @Hallo);
  WriteLn(Addr(Zeige) = Ptr, ' ', Addr(Q) = @Q);
  Op := Minus;
  WriteLn(TBinOp(@Plus)(1, 2), ' ', Op(1, 2))
end.
