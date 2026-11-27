{ Parameterlose Prozedurvariablen als Anweisung: über Index, Zeiger,
  Recordfeld und with; parameterlose Funktionsvariablen im Ausdruck }
program Anweisung;

type
  TProc = procedure;
  TZahl = function: Integer;
  PProc = ^TProc;
  TTabelle = record
    Start: TProc;
    Wert: TZahl
  end;

var
  A: array[1..2] of TProc;
  PP: PProc;
  R: TTabelle;
  Z: array[1..2] of TZahl;
  I: Integer;

procedure Eins;
begin
  WriteLn('eins')
end;

procedure Zwei;
begin
  WriteLn('zwei')
end;

function Sieben: Integer;
begin
  Sieben := 7
end;

function Drei: Integer;
begin
  Drei := 3
end;

begin
  A[1] := Eins;
  A[2] := Zwei;
  for I := 2 downto 1 do
    A[I];
  New(PP);
  PP^ := Zwei;
  PP^;
  R.Start := Eins;
  R.Wert := Sieben;
  R.Start;
  WriteLn(R.Wert * 2);
  with R do
  begin
    Start;
    WriteLn(Wert + 1)
  end;
  Z[1] := Drei;
  Z[2] := R.Wert;
  WriteLn(Z[1] + Z[2]);
  Dispose(PP)
end.
