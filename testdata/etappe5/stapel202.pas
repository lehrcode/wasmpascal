program Stapel;
{$M 16384,0,655360}
{ Mit nur 16 KB Schatten-Stack laeuft die Rekursion ueber. }
var
  Tiefe: Integer;

procedure Rekursion(n: Integer);
var
  Puffer: array[1..1000] of Byte;
begin
  Puffer[1] := n mod 256;
  Tiefe := n;
  Rekursion(n + 1)
end;

begin
  WriteLn('Start');
  Rekursion(1)
end.
